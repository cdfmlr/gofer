package gofer

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
)

// SimpleFile 是简单的文件。
//
// SimpleFile is Packet that:
//  - Type: 3
//  - Info: string, `fileName`
//  - Data: string, `fileContent`
//
// SimpleFile 在构建的过程中会把整个文件读取到 Packet.Data 中，因而只试用于很小的文件。
type SimpleFile struct {
	*Packet
	fileName    string // just a name, do not use this, call Getter/Setter instead
	fileContent []byte // just a name, do not use this, call Getter/Setter instead
}

const PacketTypeSimpleFile uint16 = 3

func NewSimpleFile(filePath string) *SimpleFile {
	fileName := filepath.Base(filePath)

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Open file failed:", err)
		return &SimpleFile{}
	}
	defer file.Close()

	// Read file
	fileContent, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("Read file failed:", err)
		return &SimpleFile{}
	}

	return &SimpleFile{
		Packet: NewPacket(PacketTypeSimpleFile, []byte(fileName), fileContent),
	}
}

// PacketAsSimpleFile convert packet to SimpleFile
// Notice: a packet should be convert to SimpleFile if and only if its Type==PacketTypeSimpleFile
func PacketAsSimpleFile(packet *Packet) *SimpleFile {
	return &SimpleFile{Packet: packet}
}

func (s SimpleFile) FileName() string {
	return string(s.Info)
}

func (s *SimpleFile) SetFileName(fileName string) {
	s.Info = []byte(fileName)
	s.InfoSize = uint32(len(s.Info))
}

func (s SimpleFile) FileContent() []byte {
	return s.Data
}

func (s *SimpleFile) SetFileContent(fileContent []byte) {
	s.Data = fileContent
	s.DataSize = uint32(len(s.Data))
}

// WriteFileContent 把 FileContent 写入到 writer 里
func (s SimpleFile) WriteFileContent(writer io.Writer) (written int, err error) {
	return writer.Write(s.FileContent())
}

// SimpleFileSender 负责发一个文件
type SimpleFileSender struct {
	simpleFile *SimpleFile
}

func NewSimpleFileSender(filePath string) *SimpleFileSender {
	return &SimpleFileSender{
		simpleFile: NewSimpleFile(filePath),
	}
}

func (s SimpleFileSender) Send(conn net.Conn) {
	n, err := s.simpleFile.WriteTo(conn)

	if err != nil {
		fmt.Println("simpleFile send failed:", err)
	} else {
		fmt.Println("simpleFile sent successfully: length =", n)
	}
}

// SimpleFileSender 负责处理一个接收到的 SimpleFile 类型的 Packet
type SimpleFileReceiver struct{}

func NewSimpleFileReceiver() *SimpleFileReceiver {
	return &SimpleFileReceiver{}
}

func (s SimpleFileReceiver) Receive(packet *Packet, conn net.Conn) chan bool {
	done := make(chan bool, 1)

	if packet.Type != PacketTypeSimpleFile {
		log.Fatal("SimpleFileReceiver got a no SimpleFile packet:", packet.Header)
	}
	sf := PacketAsSimpleFile(packet)

	file, err := os.OpenFile(sf.FileName(), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		fmt.Printf("[SimpleFile] %s: save failed: %v\n", sf.FileName(), err)
		done <- false
		return done
	}
	defer file.Close()

	if _, err := sf.WriteFileContent(file); err != nil {
		fmt.Printf("[SimpleFile] %s: save failed: %v\n", sf.FileName(), err)
		done <- false
		return done
	}

	fmt.Printf("[SimpleFile] %s: %d Bytes saved.\n", sf.FileName(), sf.DataSize)

	done <- true
	return done
}

// 注册 SimpleFileSender
func init() {
	DistributerInstance().Register(PacketTypeSimpleFile, NewSimpleFileReceiver())
}
