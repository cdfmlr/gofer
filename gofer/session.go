package gofer

//
//import "net"
//
//// session 是一个传输的会话连接
//type session struct {
//	conn net.Conn
//}
//
//// NewSession 从网络连接新建一个 session
//func NewSession(conn net.Conn) *session {
//	return &session{conn: conn}
//}
//
//// Send 发送数据包
//func (s *session) Send(packet *Packet) (written int, err error) {
//	return packet.WriteTo(s.conn)
//}
//
//// Receive 接收数据包
//func (s *session) Receive() (packet *Packet, err error) {
//	return PacketFromReader(s.conn)
//}
//
//// RemoteAddrString 返回对方连接地址
//func (s session) RemoteAddr() net.Addr {
//	return s.conn.RemoteAddr()
//}
