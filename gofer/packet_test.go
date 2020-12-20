package gofer

import (
	"fmt"
	"os"
	"testing"
)

func TestPacketToMessage(t *testing.T) {
	i := NewPacket(2, []byte("Info"), []byte("This is data with 中文"))
	message := Message{Packet: i}
	fmt.Println(message.GetContent())
	fmt.Println(message.ToBytes())
	message.WriteTo(os.Stdout)
}
