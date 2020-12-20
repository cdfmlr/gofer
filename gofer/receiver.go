package gofer

import (
	"fmt"
	"net"
)

// PacketReceiver 处理接收到的 Packet
//
// 其实应该叫做 Handler，但为了呼应 Sender，就叫做 PacketReceiver 了。
type PacketReceiver interface {
	Receive(packet *Packet, conn net.Conn) chan bool // 收完了就往里面传值
}

// Receiver 负责从 conn 接收 packet, 然后分发给各种 PacketReceiver 处理
//
// ⚠️ 注意：
//    Receiver 不是 PacketReceiver。Receiver 位于更底层，是和网络连接的 conn、
//    网络中的数据流打交道的，而 PacketReceiver 只和抽象封装的 Packet 对象打交道。
type Receiver struct {
	Distributer *Distributer
}

func NewReceiver() *Receiver {
	return &Receiver{Distributer: DistributerInstance()}
}

// ReceiveAndHandle 接收并处理数据包
func (r Receiver) ReceiveAndHandle(conn net.Conn) chan bool {
	packet, err := PacketFromReader(conn)

	if err != nil {
		fmt.Println("receive from", conn.RemoteAddr().String(), "failed:", err)
	} else {
		fmt.Println("receive from", conn.RemoteAddr().String(), "success")
	}

	return r.Distributer.Receive(packet, conn)
}
