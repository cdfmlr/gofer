package gofer

import (
	"log"
	"net"
	"sync"
)

// Distributer 是一个 PacketReceiver
//
// 所有 Receiver 接收到的 packet 都会交给这个东西，由这个东西分发给其他 PacketReceiver 具体处理
// 其他所有的 PacketReceiver 都应该在这里注册。
type Distributer struct {
	packetReceivers sync.Map
}

// Register 注册一个 PacketReceiver
func (d *Distributer) Register(packetType uint16, packetReceiver PacketReceiver) {
	d.packetReceivers.Store(packetType, packetReceiver)
}

// Receive 完成 Distributer 的分发工作
func (d *Distributer) Receive(packet *Packet, conn net.Conn) chan bool {
	done := make(chan bool, 1)

	packetReceiver, ok := d.packetReceivers.Load(packet.Type)
	if !ok { // 没有接收的处理器，默认处理
		log.Printf("Got an unknown Packet: %#v", packet)
		done <- false
		return done
	}

	return packetReceiver.(PacketReceiver).Receive(packet, conn)
}

// Distributer 在程序中保持 Distributer 单例会比较方便（便于各种 PacketReceiver 的注册）
var _distributer = Distributer{}

// DistributerInstance 获取 Distributer 单例
func DistributerInstance() *Distributer {
	return &_distributer
}
