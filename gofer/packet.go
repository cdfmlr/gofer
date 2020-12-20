package gofer

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Packet 是数据传输的一个"数据包"。
//
// 相当于一个自定义的协议:
//
//  |      |                 Header                 |         Body        |
//  | ---- | ---- - -------- - -------- - --------- | -------- - -------- |
//  | PART | Type | InfoSize | DataSize | RESERVED* |   Info   |   Data   |
//  | ---- | ---- | -------- | -------- | --------- | -------- | -------- |
//  | SIZE |  2B  |    4B    |    4B    |    2B     | InfoSize | DataSize |
//
//  * RESERVED: 保留字段, 暂时没用. 只是让 Header 对齐到 8/2 的整数倍。
//
// Packet 主要由两部分数据组成: Header 和 Body:
//
// Header 是固定长度的描述区域:
//  - Type 描述数据的类型, 例如 2 表示消息, 3 表示文件
//  - InfoSize 和 DataSize, 描述 Info 和 Data 的长度 (in Bytes)
// Body 是长度不确定的数据区域:
//  - Info 是数据的特定信息, 例如 Type 为文件时, Info 可能就是文件名
//  - Data 即主体数据, 例如消息内容、文件内容...
type Packet struct {
	Header
	Body
}

// Header 是 Packet 中固定长度 (20 Byte) 的描述区域:
//  - Type 描述数据的类型, 例如 2 表示消息, 3 表示文件
//  - InfoSize 和 DataSize, 描述 Info 和 Data 的长度 (in Bytes)
type Header struct {
	// Type: 数据类型, 固定 2 Byte
	Type uint16
	// InfoSize: 描述信息的大小, 固定 4 Byte
	InfoSize uint32
	// DataSize: 数据的大小, 固定 4 Byte
	DataSize uint32
}

// Body 是 Packet 中长度不确定的数据区域:
//  - Info 是数据的特定信息, 例如 Type 为文件时, Info 可能就是文件名
//  - Data 即主体数据, 例如消息内容、文件内容...
type Body struct {
	// Info: 数据的描述, 数据长度由指定 InfoSize 指定
	Info []byte
	// Data: 数据, 长度由指定 DataSize 指定
	Data []byte
}

func NewPacket(typ uint16, info []byte, data []byte) *Packet {
	return &Packet{
		Header: Header{
			Type:     typ,
			InfoSize: uint32(len(info)),
			DataSize: uint32(len(data)),
		},
		Body: Body{
			Info: info,
			Data: data,
		},
	}
}

// ToBytes encodes Packet => []byte
func (p *Packet) ToBytes() []byte {
	totalSize := 2 + 4 + 4 + 2 + p.InfoSize + p.DataSize
	buf := make([]byte, totalSize)

	// Type: [0, 2)
	binary.BigEndian.PutUint16(buf[:2], p.Type)

	// InfoSize: [2, 6)
	binary.BigEndian.PutUint32(buf[2:6], p.InfoSize)

	// DataSize: [6, 10)
	binary.BigEndian.PutUint32(buf[6:10], p.DataSize)

	// RESERVED: [10, 12)

	// Info: [12, 12+p.InfoSize)
	copy(buf[12:12+p.InfoSize], p.Info)

	// Data: [12+p.InfoSize, 12+p.InfoSize+p.DataSize)
	copy(buf[12+p.InfoSize:], p.Data)

	return buf
}

// WriteTo 把 Packet 写到一个 writer 里
func (p *Packet) WriteTo(writer io.Writer) (written int, err error) {
	return writer.Write(p.ToBytes())
}

// fillHeaderFromBytes 从 header 中读取信息填充到 packet 里
func (p *Packet) fillHeaderFromBytes(header []byte) error {
	if len(header) < 12 {
		return fmt.Errorf("ValueError: b too short")
	}

	p.Type = binary.BigEndian.Uint16(header[:2])
	p.InfoSize = binary.BigEndian.Uint32(header[2:6])
	p.DataSize = binary.BigEndian.Uint32(header[6:10])

	return nil
}

// PacketFromBytes decodes []byte => Packet
func PacketFromBytes(b []byte) (*Packet, error) {
	// XXX: 这个方法可以直接借助 PacketFromReader
	// return PacketFromReader(bytes.NewBuffer(b))

	p := &Packet{}

	// Header
	if err := p.fillHeaderFromBytes(b); err != nil {
		return nil, err
	}

	// Body
	if uint32(len(b)) < 12+p.InfoSize+p.DataSize {
		return nil, fmt.Errorf("ValueError: b shorter than described")
	}
	p.Info = b[12 : 12+p.InfoSize]
	p.Data = b[12+p.InfoSize : 12+p.InfoSize+p.DataSize]

	return p, nil
}

// PacketFromReader 从 reader 中读取字节, 解码成 Packet
func PacketFromReader(reader io.Reader) (*Packet, error) {
	p := &Packet{}

	// Header
	header := make([]byte, 12)
	if _, err := io.ReadFull(reader, header); err != nil {
		return p, err
	}
	// 如果上面 ReadFull 成功的话, fillHeaderFromBytes 是不会出错的, 所以没有必要检测错误
	// if err := packet.fillHeaderFromBytes(header); err != nil {
	// 	return nil, err
	// }
	_ = p.fillHeaderFromBytes(header)

	// Info
	info := make([]byte, p.InfoSize)
	if _, err := io.ReadFull(reader, info); err != nil {
		return p, err
	}
	p.Info = info

	// Data
	data := make([]byte, p.DataSize)
	if _, err := io.ReadFull(reader, data); err != nil {
		return p, err
	}
	p.Data = data

	return p, nil
}
