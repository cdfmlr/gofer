package gofer

import (
	"fmt"
	"log"
	"net"
)

// Message 表示一条即时通讯的消息
//
// Message is Packet that:
//  - Type: 2
//  - Info: string, message `info`
//  - Data: string, message `content`
type Message struct {
	*Packet
	info    string // just a name, do not use this, call Getter/Setter instead
	content string // just a name, do not use this, call Getter/Setter instead
}

const PacketTypeMessage uint16 = 2

func NewMessage(info string, content string) *Message {
	return &Message{
		Packet: NewPacket(PacketTypeMessage, []byte(info), []byte(content)),
	}
}

// PacketAsMessage convert packet to message
// Notice: a packet should be convert to message if and only if its Type==PacketTypeMessage
func PacketAsMessage(packet *Packet) *Message {
	return &Message{Packet: packet}
}

func (m Message) GetInfo() string {
	return string(m.Info)
}

func (m *Message) SetInfo(info string) {
	m.Info = []byte(info)
	m.InfoSize = uint32(len(m.Info))
}

func (m Message) GetContent() string {
	return string(m.Data)
}

func (m *Message) SetContent(content string) {
	m.Data = []byte(content)
	m.DataSize = uint32(len(m.Data))
}

// MessageSender 是用来发一条消息的东西
type MessageSender struct {
	message *Message
}

func NewMessageSender(msgInfo string, msgContent string) *MessageSender {
	return &MessageSender{
		message: NewMessage(msgInfo, msgContent),
	}
}

func (m MessageSender) Send(conn net.Conn) {
	n, err := m.message.WriteTo(conn)

	if err != nil {
		fmt.Println("message send failed:", err)
	} else {
		fmt.Println("message sent successfully: length =", n)
	}
}

// MessageReceiver 是接收一条消息并处理的东西
type MessageReceiver struct{}

func NewMessageReceiver() *MessageReceiver {
	return &MessageReceiver{}
}

func (m MessageReceiver) Receive(packet *Packet, conn net.Conn) chan bool {
	if packet.Type != PacketTypeMessage {
		log.Fatal("MessageReceiver got a no message packet:", packet.Header)
	}
	msg := PacketAsMessage(packet)
	fmt.Printf("[Message] %s: %s\n", msg.GetInfo(), msg.GetContent())

	done := make(chan bool, 1)
	done <- true
	return done
}

// 注册 MessageReceiver
func init() {
	DistributerInstance().Register(PacketTypeMessage, NewMessageReceiver())
}
