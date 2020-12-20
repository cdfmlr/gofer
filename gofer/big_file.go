package gofer

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const DefaultBlockSize = 1 * 1024 * 1024 // 1 MiB

// 大文件！！
// 总而言之就是很大的文件, 不容易一次正确、快速传完的那种。
//
// 面对这种洪水猛兽，这里采取的对策是：
//
//「发送端」先把文件信息给「接收端」，其中包含文件大小，
//然后由「接收端」一段一段地请求下载，最后合成一个文件。
//
//这样也就支持了断点续传、并发处理。
//
//定义如下数据结构:
//
// - BigFileHeader
// - BigFileRequest
// - BigFileResponse
//
// - BigFileSender
// - BigFileReceiver
//
//发送大文件的流程就可以表示为:
//
//  1. BigFileSender 读取大文件信息，构建 BigFileHeader
//  2. BigFileSender 把 BigFileHeader 发给 BigFileReceiver
//  3. BigFileReceiver 发送 BigFileRequest 给 BigFileSender，请求下载一段文件
//  4. BigFileSender 把请求的文件段写入 BigFileResponse 发给 BigFileReceiver
//  5. BigFileReceiver 把 BigFileResponse 收到的文件部分写入磁盘
//  6. 重复 3~5, 直到 BigFileReceiver 接收到全部文件部分, 然后合并、校验文件。
//  7. BigFileReceiver 回传 BigFileHeader 给 BigFileSender，表示接收完成。
//  8. BigFileReceiver out, BigFileSender out. Done! 🎉
//

// BigFileHeader 是大文件 sender 发给 Receiver 的文件信息说明。
// 包含传输过程需要的一些关键属性：
//  - fileID  : 文件ID，用来在后面的传输过程中表识文件，具体的实现是文件的 md5 和
//  - fileName: 文件名
//  - fileSize: 文件大小, 单位是 Byte
//  - fileHash: 文件摘要，用来做最终校验，实现上其实就是 fileID
//
// BigFileHeader is Packet that:
//  - Type: 4
//  - Info: fileID(fileHash)
//  - Data: fileSize (const 8 Byte), fileName
type BigFileHeader struct {
	*Packet
	fileID   []byte // just a name, do not use this, call Getter/Setter instead
	fileName string // just a name, do not use this, call Getter/Setter instead
	fileSize uint64 // just a name, do not use this, call Getter/Setter instead
	fileHash []byte // just a name, do not use this, call Getter/Setter instead
}

const PacketTypeBigFileHeader = 4

func NewBigFileHeader(fileID []byte, fileName string, fileSize uint64) *BigFileHeader {
	h := &BigFileHeader{Packet: NewPacket(PacketTypeBigFileHeader, make([]byte, 0), make([]byte, 0))}

	h.SetFileID(fileID)
	h.SetFileName(fileName)
	h.SetFileSize(fileSize)

	//log.Printf("[Debug] NewBigFileHeader: %v %v %v", h.FileID(), h.FileName(), h.FileSize())

	return h
}

// PacketAsBigFileHeader convert packet to BigFileHeader
// Notice: only for packets whose Type==PacketTypeBigFileHeader
func PacketAsBigFileHeader(packet *Packet) *BigFileHeader {
	return &BigFileHeader{Packet: packet}
}

func (b *BigFileHeader) FileID() []byte {
	return b.Info
}

func (b *BigFileHeader) SetFileID(fileID []byte) {
	b.Info = fileID
	b.InfoSize = uint32(len(fileID))
}

func (b *BigFileHeader) FileName() string {
	return string(b.Data[8:]) // 前 8 Byte 是 uint64 的 fileSize
}

func (b *BigFileHeader) SetFileName(fileName string) {
	b.DataSize = uint32(8 + len(fileName))
	buf := make([]byte, b.DataSize)
	if len(b.Data) >= 8 {
		copy(buf, b.Data[:8])
	}
	copy(buf[8:], []byte(fileName))
	b.Data = buf
}

func (b *BigFileHeader) FileSize() uint64 {
	return binary.BigEndian.Uint64(b.Data[:8])
}

func (b *BigFileHeader) SetFileSize(fileSize uint64) {
	if b.DataSize < 8 {
		b.DataSize = 8
		b.Data = make([]byte, 8)
	}
	binary.BigEndian.PutUint64(b.Data[:8], fileSize)
}

func (b *BigFileHeader) FileHash() []byte {
	return b.FileID()
}

func (b *BigFileHeader) SetFileHash(fileHash []byte) {
	b.SetFileID(fileHash)
}

// BigFileRequest 是大文件的 Receiver 发给 sender 的文件段请求。
//
// Receiver 通过发送 BigFileRequest 给 sender 来请求某一段文件。
// 所以说, BigFileRequest 中包括请求文件的 fileID 以及文件段的起始
// 位置 start (字节数) 和请求下载长度 length (字节数)。
//
// BigFileRequest is Packet that:
//  - Type: 5
//  - Info: fileID
//  - Data: start(const 8 Byte), length(const 8 Byte)
type BigFileRequest struct {
	*Packet
	fileID []byte // just a name, do not use this, call Getter/Setter instead
	start  uint64 // just a name, do not use this, call Getter/Setter instead
	length uint64 // just a name, do not use this, call Getter/Setter instead
}

const PacketTypeBigFileRequest = 5

func NewBigFileRequest(fileID []byte, start uint64, length uint64) *BigFileRequest {
	r := &BigFileRequest{Packet: NewPacket(PacketTypeBigFileRequest, make([]byte, 0), make([]byte, 0))}
	r.SetFileID(fileID)
	r.SetStart(start)
	r.SetLength(length)

	//log.Println("[Debug] NewBigFileRequest:", r.FileID(), r.Start(), r.Length())

	return r
}

// PacketAsBigFileRequest convert packet to BigFileRequest
// Notice: only for packets whose Type==PacketTypeBigFileRequest
func PacketAsBigFileRequest(packet *Packet) *BigFileRequest {
	return &BigFileRequest{Packet: packet}
}

func (b *BigFileRequest) FileID() []byte {
	return b.Info
}

func (b *BigFileRequest) SetFileID(fileID []byte) {
	b.Info = fileID
	b.InfoSize = uint32(len(b.Info))
}

func (b *BigFileRequest) Start() uint64 {
	return binary.BigEndian.Uint64(b.Data[:8])
}

func (b *BigFileRequest) SetStart(start uint64) {
	if len(b.Data) < 16 {
		b.Data = make([]byte, 16)
		b.DataSize = 16
	}
	binary.BigEndian.PutUint64(b.Data[:8], start)
}

func (b *BigFileRequest) Length() uint64 {
	return binary.BigEndian.Uint64(b.Data[8:])
}

func (b *BigFileRequest) SetLength(length uint64) {
	if len(b.Data) < 16 {
		b.Data = make([]byte, 16)
		b.DataSize = 16
	}
	binary.BigEndian.PutUint64(b.Data[8:], length)
}

// BigFileResponse 是大文件的 sender 发给 Receiver 的文件段
//
//  - fileID: 文件 ID
//  - start: 起始位置
//  - fileContent: 文件段的内容
//
// BigFileResponse is Packet that:
//  - Type: 6
//  - Info: start(const 8 Byte), fileID
//  - Data: fileContent
type BigFileResponse struct {
	*Packet
	fileID      []byte // just a name, do not use this, call Getter/Setter instead
	start       uint64 // just a name, do not use this, call Getter/Setter instead
	fileContent []byte // just a name, do not use this, call Getter/Setter instead
}

func NewBigFileResponse(fileID []byte, start uint64, fileContent []byte) *BigFileResponse {
	r := &BigFileResponse{Packet: NewPacket(PacketTypeBigFileResponse, make([]byte, 0), make([]byte, 0))}
	r.SetFileID(fileID)
	r.SetStart(start)
	r.SetFileContent(fileContent)
	return r
}

const PacketTypeBigFileResponse = 6

// PacketAsBigFileResponse convert packet to BigFileResponse
// Notice: only for packets whose Type==PacketTypeBigFileResponse
func PacketAsBigFileResponse(packet *Packet) *BigFileResponse {
	return &BigFileResponse{Packet: packet}
}

func (b *BigFileResponse) FileContent() []byte {
	return b.Data
}

func (b *BigFileResponse) SetFileContent(fileContent []byte) {
	b.Data = fileContent
	b.DataSize = uint32(len(b.Data))
}

func (b *BigFileResponse) Start() uint64 {
	return binary.BigEndian.Uint64(b.Info[:8])
}

func (b *BigFileResponse) SetStart(start uint64) {
	if b.InfoSize < 8 {
		b.InfoSize = 8
		b.Info = make([]byte, 8)
	}
	binary.BigEndian.PutUint64(b.Info[:8], start)
}

func (b *BigFileResponse) FileID() []byte {
	return b.Info[8:]
}

func (b *BigFileResponse) SetFileID(fileID []byte) {
	b.InfoSize = uint32(8 + len(fileID))
	buf := make([]byte, b.InfoSize)
	if len(b.Info) >= 8 {
		copy(buf, b.Info[:8])
	}
	copy(buf[8:], fileID)
	b.Info = buf
}

// FileIDString returns Sprintf("%x", fileID)
func FileIDString(fileID []byte) string {
	return fmt.Sprintf("%x", fileID)
}

// BigFileSender 是发大文件用的东西
// 实现 Sender 接口
//
// 和 Message、SimpleFile 那种不同, BigFileSender 其实是一个"服务"了，
// 它监听 conn, 从里面读请求（BigFileRequest），写响应（BigFileResponse）
type BigFileSender struct {
	filePathMap sync.Map // {fileIDString: "path/to/file"}
	headerMap   sync.Map // {fileIDString: BigFileHeader}
}

func NewBigFileSender() *BigFileSender {
	return &BigFileSender{}
}

func (s *BigFileSender) AppendFile(filePath string) {
	fileName := filepath.Base(filePath)
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Open file failed:", err)
		return
	}
	defer file.Close()

	// Get hash and size
	h := md5.New()
	fileSize, err := io.Copy(h, file)
	if err != nil {
		log.Fatal(err)
	}
	fileHash := h.Sum(nil)

	// Store
	fileIDString := FileIDString(fileHash)

	//log.Println("[Debug] AppendFile:", fileIDString, filePath, fileName, fileSize)

	s.filePathMap.Store(fileIDString, filePath)
	s.headerMap.Store(fileIDString, *NewBigFileHeader(fileHash, fileName, uint64(fileSize)))
}

// Send 向 conn 发送一次头（sendHeader），然后调用 sendResponse 监听 conn,
// 从里面读请求（BigFileRequest），写响应（BigFileResponse）；
// 如果长时间没有请求则重新使用 sendHeader 发送 Header
func (s BigFileSender) Send(conn net.Conn) {
	// TODO: 错误时通知请求者
	s.sendHeader(conn)
	resp := s.sendResponse(conn)
	for {
		select {
		case ok := <-resp:
			if ok {
				return
			}
		case <-time.After(3 * time.Second):
			s.sendHeader(conn)
		}
	}
}

// sendResponse 监听 conn, 从里面读请求（BigFileRequest），写响应（BigFileResponse）
func (s BigFileSender) sendResponse(conn net.Conn) chan bool {
	done := make(chan bool)

	go func() {
		for {
			// 获取请求
			packet, err := PacketFromReader(conn)
			//log.Println("[DEBUG] sendResponse, got from conn:", packet.Header)
			if err != nil {
				log.Fatal(err)
			}
			if packet.Type == PacketTypeBigFileHeader { // Receiver 回传 header，文件发送结束
				header := PacketAsBigFileHeader(packet)
				fileIDString := FileIDString(header.FileID())

				log.Println("BigFileSender: over:", fileIDString)

				done <- true // 这才是真的发完了，回传 true，结束工作
				return
			}
			if packet.Type != PacketTypeBigFileRequest {
				log.Println("BigFileSender: req.Type != PacketTypeBigFileRequest:", packet.Header)
				done <- false
				return
			}
			req := PacketAsBigFileRequest(packet)

			// 获取响应
			resp, err := s.responseReq(req)
			if err != nil {
				log.Println("BigFile response failed:", err)
				done <- false
				return
			}

			//log.Println("[debug] sendResponse:", FileIDString(resp.FileID()), resp.DataSize)

			// 发送响应
			if n, err := resp.WriteTo(conn); err != nil {
				fmt.Println("BigFile send failed:", err)
			} else {
				fmt.Println("BigFile sent successfully: length =", n)
			}

			done <- false // 还有继续呀，所以是 false
		}
	}()

	return done
}

// responseReq 解析 BigFileRequest 的请求，构造 BigFileResponse
func (s BigFileSender) responseReq(req *BigFileRequest) (*BigFileResponse, error) {
	//log.Println("[Debug] responseReq", req.FileID(), req.Start())
	// Open file
	filePath, ok := s.filePathMap.Load(FileIDString(req.FileID()))
	if !ok {
		return nil, fmt.Errorf("bigFileSender: resource not found")
	}

	file, err := os.Open(filePath.(string))
	if err != nil {
		return nil, fmt.Errorf("bigFileSender: resource not found: %v", err)
	}
	defer file.Close()

	// Read
	buf := make([]byte, req.Length())
	n, err := file.ReadAt(buf, int64(req.Start()))
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("bigFileSender: read file error: %v", err)
	}

	// make a response
	resp := NewBigFileResponse(req.FileID(), req.Start(), buf[:n])

	log.Printf("[bigFileSender] response: flie=%s offset=%v length=%v",
		filePath, req.Start(), n)

	return resp, nil
}

// sendHeader 向 conn 发送一次 BigFileHeader
func (s BigFileSender) sendHeader(conn net.Conn) {
	s.headerMap.Range(func(key, value interface{}) bool {
		header := value.(BigFileHeader)
		//log.Printf("[Debug] BigFileSender.sendHeader: %v %v %v", header.FileID(), header.FileName(), header.FileSize())
		_, _ = header.WriteTo(conn)
		return true
	})
}

// BigFileSender 是收大文件用的东西:
// 实现了 PacketReceiver 接口
//
// 接收大文件这里设计成用 Master-Worker 模式：
// BigFileReceiver 是 Master, 只是指派、管理工作;
// 而具体的文件下载工作由 BigFileReceiverWorker 来做。
type BigFileReceiver struct {
	workerMap sync.Map // {FileIDString(fileID): BigFileReceiverWorker}
	wg        sync.WaitGroup
}

func NewBigFileReceiver() *BigFileReceiver {
	//log.Println("[DEBUG] NewBigFileReceiver")
	return &BigFileReceiver{}
}

// Receive 处理接收到的 BigFileHeader 和 BigFileResponse
// 分发给 handleBigFileHeader 和 handleBigFileResponse 方法处理
func (r *BigFileReceiver) Receive(packet *Packet, conn net.Conn) chan bool {
	done := make(chan bool, 1)

	//log.Printf("[Debug] BigFileReceiver(%p).Receive: %v", r, packet)
	switch packet.Type {
	case PacketTypeBigFileHeader:
		r.handleBigFileHeader(PacketAsBigFileHeader(packet), conn)
	case PacketTypeBigFileResponse:
		r.handleBigFileResponse(PacketAsBigFileResponse(packet), conn)
	default:
		log.Printf("BigFileReceiver got an unknown Packet: %#v", packet)
	}

	go func() {
		r.wg.Wait()
		done <- true
	}()

	return done // 有 worker 就还要继续
}

// handleBigFileHeader 处理收到的大文件头:
// 新建一个 worker 去处理，结束后删除 worker。
func (r *BigFileReceiver) handleBigFileHeader(header *BigFileHeader, conn net.Conn) {
	fileID := FileIDString(header.FileID())

	//log.Println("[DEBUG] BigFileReceiver handleBigFileHeader:", fileID)

	_, ok := r.workerMap.Load(fileID)
	if ok { // file is already exist
		//log.Println("handleBigFileHeader: file is already on receiving")
		return
	}

	worker := NewBigFileReceiverWorker(header)
	r.wg.Add(1)
	r.workerMap.Store(fileID, worker)
	//_h, _ok := r.workerMap.Load(fileID)
	//log.Println("[DEBUG] handleBigFileHeader:", r.workerMap, r)
	//log.Printf("[DEBUG] handleBigFileHeader: %p %p", &r.workerMap, r)
	workerDone := worker.Run(conn)

	go func() { // cleanup
		select {
		case f := <-workerDone:
			//log.Println("[DEBUG] workerDone:", f)
			r.workerMap.Delete(f)
			r.wg.Done()
		}
	}()
}

// handleBigFileResponse 处理收到的 BigFileResponse：
// 找到对应的 worker 去处理
func (r *BigFileReceiver) handleBigFileResponse(response *BigFileResponse, conn net.Conn) {
	fileIDString := FileIDString(response.FileID())
	//log.Println("[DEBUG] BigFileReceiver handleBigFileResponse:", fileIDString)
	//log.Printf("[DEBUG] BigFileReceiver handleBigFileResponse: %p %p", &r.workerMap, r)
	worker, ok := r.workerMap.Load(fileIDString)
	if !ok { // worker is not exist
		log.Println("BigFileReceiver: worker not found: fileID =", fileIDString)
		return
	}
	worker.(*BigFileReceiverWorker).Receive(response)
}

// BigFileReceiverWorker 负责接收一个大文件。
//
// 一个 Worker 只专注处理一个大文件。
// BigFileReceiver 通过把 BigFileHeader 指派给 Worker，让 Worker 自行处理一个大文件的下载工作。
//
// Worker 会在 $PWD 新建一个以 ".{fileID}" 为名的目录（称为 saveDir），
// 每次请求下载 blockSize 大小的文件片段, 保存到 saveDir, 文件名为 "{blockIndex}.block",
// 把 savedBlock 中对应的块位置标记为 1。
//
// 重复下载过程，直到 savedBlock 全为 1，然后合并文件，计算 md5 和，检查是否正确。
// 正确则 mv mergedFile $PWD/{fileName}, 不正确就丢弃。
//
// 断点续传: Worker 并不是直接新建 saveDir。如果 saveDir 存在，则打开，
// 从里面读取已保存的文件片段，更新 savedBlock，然后再开始下载缺失部分。
type BigFileReceiverWorker struct {
	header     *BigFileHeader // 大文件头
	saveDir    string         // 临时目录的保存路径
	blockSize  uint64         // 块大小, XXX: 第一个版本为了方便，固定 blockSize 为 DefaultBlockSize
	numBlock   uint64         // 块数量
	savedBlock []bool         // bitmap: 已保存块为 1，未保存的为 0
	done       chan string    // worker 工作结束后通知 master (BigFileReceiver), 或 master 来终止 worker
	wait       chan int       // 请求下载之后等待接收, 值是 blockIndex
	allSaved   chan bool      // 所有部分都下载完成了
	// XXX: 第一个版本并发度不高，暂时不加锁了，鸵鸟算法凑合一下
}

func NewBigFileReceiverWorker(header *BigFileHeader) *BigFileReceiverWorker {
	var blockSize uint64 = DefaultBlockSize
	return &BigFileReceiverWorker{
		header:    header,
		blockSize: blockSize,
	}
}

// init 读取/新建 saveDir, 设置 numBlock、savedBlock bitmap
func (w *BigFileReceiverWorker) init() {
	// 初始化 numBlock、savedBlock
	w.numBlock = w._numBlock()
	w.savedBlock = make([]bool, w.numBlock)

	// 检查 saveDir, 读取 or 新建
	w.saveDir = w._saveDir()
	w.prepareSaveDir()

	// 同步 savedBlock 和 saveDir 里的真实情况
	w.checkSaved()
}

// _numBlock 计算正确的块数 NumBlock，返回结果。
// 注意，这个方法不设置 numBlock 字段, 要设置的话请手动赋值:
//    w.numBlock = w.getNumBlock()
func (w *BigFileReceiverWorker) _numBlock() uint64 {
	n := w.header.FileSize() / w.blockSize
	r := w.header.FileSize() % w.blockSize
	if r > 0 {
		n += 1
	}
	return n
}

// _saveDir 计算正确的临时保存路径 saveDir，返回结果。
// 注意，这个方法不设置 saveDir 字段, 要设置的话请手动赋值.
func (w *BigFileReceiverWorker) _saveDir() string {
	return fmt.Sprintf(".%s", FileIDString(w.header.FileID()))
}

// prepareSaveDir 准备 w.saveDir
// 也就是检查目录存不存在啦，不存在就新建
func (w *BigFileReceiverWorker) prepareSaveDir() {
	s, err := os.Stat(w.saveDir)
	if err != nil && os.IsNotExist(err) { // 不存在
		if err = os.Mkdir(w.saveDir, 0755); err != nil { // 新建
			log.Fatal("BigFileReceiverWorker failed to make saveDir:", err)
		}
	} else if !s.IsDir() {
		log.Fatalf("BigFileReceiverWorker failed to use saveDir:"+
			" Please remove this file first: %s", w.saveDir)
	}
}

// checkSaved 遍历 w.saveDir 里的文件，找出已经下载了那些文件片段，标记到 w.savedBlock
func (w *BigFileReceiverWorker) checkSaved() {
	filepath.Walk(w.saveDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".block") {
			var p int
			fmt.Sscanf(info.Name(), "%d", &p)
			if p < len(w.savedBlock) {
				w.savedBlock[p] = true
			} else {
				log.Printf("Unexpected saved block: %d (numBlock=%d)", p, w.numBlock)
			}
		}
		return nil
	})
}

// Run 初始化 Worker，从 conn 请求下载所有文件片段。
// 全部下载完成后，做 merge，最后把 fileID 发到该函数返回的 chan，并回传 header 通知发送端结束工作，。
func (w *BigFileReceiverWorker) Run(conn net.Conn) chan string {
	if w.done != nil { // running
		return w.done
	}

	w.done = make(chan string)
	w.wait = make(chan int, 1) // Bug：这里不知道为什么会死锁，带上缓冲能解决；同时还收获一个 feature：缓冲给多大就同时请求几个
	w.allSaved = make(chan bool)

	w.init()

	go w.requestAllMissing(conn, w.wait, w.allSaved)

	go func() { // 等待下载全部完成后做 merge
		select {
		case <-w.allSaved:
			w.merge()
			correct := w.checkFinalSum()
			if correct {
				fmt.Println("[BigFile] receive successfully:", w.header.FileName())
			} else {
				fmt.Println("[BigFile] receive finished, but the file is BROKEN. "+
					"Please remove it and try again:", w.header.FileName())
			}

			go func() { // emmmm, 读多余的数据，防止发送端一直等待无效的发送
				_ = conn.SetReadDeadline(time.Now().Add(time.Second))
				for {
					_, err := PacketFromReader(conn)
					if err != nil {
						break
					}
				}
				_ = conn.SetReadDeadline(time.Time{})
			}()

			//n, err := w.header.WriteTo(conn) // 回传 header 通知发送端结束工作
			//log.Println("[DEBUG] header -> sender:", n, err)
			_, _ = w.header.WriteTo(conn)

			w.done <- FileIDString(w.header.FileID())
		}
	}()

	return w.done
}

// requestAllMissing 通过 conn 请求下载所有缺失（未下载）的文件段。
// 每请求一个就把 blockIndex 放到 wait 信道里，等待有人 (w.Receive 啦) 把值取走；
// 如果 wait 为 nil 则不等待。
func (w *BigFileReceiverWorker) requestAllMissing(conn net.Conn, wait chan int, allSaved chan bool) {
	var err error
	for m := w.missingBlockIndices(); len(m) > 0; m = w.missingBlockIndices() {
		for _, i := range m {
			if !w.savedBlock[i] {
				err = w.requestDownload(i, conn)
			}
			if (err == nil) && (wait != nil) {
			WAIT:
				for {
					select {
					case wait <- i:
						break WAIT
					case <-time.After(1 * time.Millisecond):
						//conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
						p, err := PacketFromReader(conn)
						//conn.SetReadDeadline(time.Time{})
						if err == nil {
							DistributerInstance().Receive(p, conn)
						}
					}
				}
			}
		}
		w.checkSaved()
	}
	allSaved <- true
}

// missingBlockIndices 返回所有没下载的块索引
func (w *BigFileReceiverWorker) missingBlockIndices() []int {
	missing := make([]int, 0)
	for i := 0; i < len(w.savedBlock); i++ {
		if !w.savedBlock[i] {
			missing = append(missing, i)
		}
	}
	return missing
}

// requestDownload 向 conn 发送下载一个文件片段的请求
func (w *BigFileReceiverWorker) requestDownload(blockIndex int, conn net.Conn) error {
	//log.Println("[DEBUG] BigFileReceiverWorker requestDownload:", blockIndex)

	start := w.blockSize * uint64(blockIndex) // offset of file

	req := NewBigFileRequest(w.header.FileID(), start, w.blockSize)

	if _, err := req.WriteTo(conn); err != nil {
		//log.Println("BigFileRequest send failed:", err)
		return err
	}
	return nil
}

// Receive 接收一个该 Worker 负责的文件的 BigFileResponse
func (w *BigFileReceiverWorker) Receive(response *BigFileResponse) {
	//log.Println("[DEBUG] BigFileReceiverWorker.Receive", FileIDString(response.FileID()), response.Start())
	if FileIDString(response.FileID()) != FileIDString(w.header.FileID()) {
		// 不是这个 worker 负责的啊，分发错了，master 不对劲🤨
		log.Fatalf("BigFileReceiverWorker (for %x) got an unexpected Response (for %x)",
			w.header.FileID(), response.FileID())
		return
	}

	block := response.Start() / w.blockSize

	err := w.saveBlock(block, response.FileContent())
	if err == nil { // 无错: 保存成功
		w.savedBlock[block] = true
	}

	// 唤醒 requestAllMissing, 继续请求
	if w.wait != nil {
		select {
		case <-w.wait:
			return
		case <-time.After(10 * time.Second): // timeout, 这个应该是个错误
			log.Println("BigFileReceiverWorker Error: Receive <-w.wait timeout")
		}
	}
}

// saveBlock 保存一个文件块
func (w *BigFileReceiverWorker) saveBlock(block uint64, fileContent []byte) error {
	name := w.BlockTmpFilePath(block)

	file, err := os.OpenFile(name, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		log.Printf("[BigFileReceiverWorker] block %v save failed: %v\n", block, err)
		return err
	}
	defer file.Close()

	if n, err := file.Write(fileContent); err != nil {
		log.Printf("[BigFileReceiverWorker] block %v save failed: %v\n", block, err)
		return err
	} else {
		log.Printf("[BigFileReceiverWorker] block %v/%v: %d Bytes saved.\n", block, w.numBlock-1, n)
	}
	return nil
}

// merge 合并文件:
// 把所有块文件逐个追加入 0.block, 然后删除, 完成后只会剩下 0.block 一个文件,
// 最后 mv saveDir/0.block $PWD/{FileName}
func (w *BigFileReceiverWorker) merge() {
	filePath0 := w.BlockTmpFilePath(0)

	mergedFile, err := os.OpenFile(filePath0, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		log.Fatalf("BigFileReceiverWorker merge fileID=%x failed: failed to open 0.block: %v",
			w.header.FileID(), err)
	}

	for i := uint64(1); i < w.numBlock; i++ {
		filePath := w.BlockTmpFilePath(i)
		file, err := os.Open(filePath)
		if err != nil {
			_ = mergedFile.Close()
			log.Fatalf("BigFileReceiverWorker merge fileID=%x failed: failed to open %d.block: %v",
				w.header.FileID(), i, err)
		}
		_, err = io.Copy(mergedFile, file)
		if err != nil {
			_ = file.Close()
			_ = mergedFile.Close()
			log.Fatalf("BigFileReceiverWorker merge fileID=%x failed: failed to merge %d.block: %v",
				w.header.FileID(), i, err)
		}

		_ = file.Close()
		_ = os.Remove(filePath)
	}

	log.Printf("[BigFileReceiverWorker] big file merge: %s => %s",
		FileIDString(w.header.FileID()), w.header.FileName())

	_ = mergedFile.Close()

	err = os.Rename(filePath0, w.header.FileName())
	if err != nil {
		log.Fatalln(err)
	}
	os.RemoveAll(w.saveDir)
}

// checkFinalSum 检查最终 merge 得到的文件 ($PWD/{FileName}) 的 md5
// 匹配则返回 true，否则 false
func (w *BigFileReceiverWorker) checkFinalSum() (ok bool) {
	f, err := os.Open(w.header.FileName())
	if err != nil {
		log.Fatalln("BigFileReceiverWorker checkFinalSum:", err)
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}
	sum := h.Sum(nil)

	return string(sum) == string(w.header.FileHash())
}

// BlockTmpFilePath 获取一个文件的一个块的临时文件路径。
// returns "{fileIDString}/{block}.block"
func (w BigFileReceiverWorker) BlockTmpFilePath(block uint64) string {
	return path.Join(
		w.saveDir,
		fmt.Sprintf("%d.block", block),
	)
}

// 注册 BigFileReceiver
func init() {
	bigFileReceiver := NewBigFileReceiver()
	DistributerInstance().Register(PacketTypeBigFileHeader, bigFileReceiver)
	DistributerInstance().Register(PacketTypeBigFileResponse, bigFileReceiver)
}
