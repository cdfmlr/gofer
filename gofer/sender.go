package gofer

import (
	"fmt"
	"net"
)

// Sender 是发送者接口
type Sender interface {
	Send(conn net.Conn)
}

// packetSender 是个通用的发 Packet 的 Sender
// 这只是个示例，实际里该不应采用。
// 应该对每种不同的 Packet 定义独特的 Sender
type packetSender struct {
	PacketToSend *Packet
}

func (s packetSender) Send(conn net.Conn) {
	n, err := s.PacketToSend.WriteTo(conn)

	if err != nil {
		fmt.Println("send failed:", err)
	} else {
		fmt.Println("sent successfully:", n)
	}
}
