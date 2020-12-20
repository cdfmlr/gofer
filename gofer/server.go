package gofer

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"
)

// Server 服务器接口
type Server interface {
	ServeConn(conn net.Conn)
}

type server struct {
	Handler Server
}

// Serve handles requests on incoming connections.
func (s *server) Serve(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Serve listener accept error:", err)
			continue
		}
		go s.Handler.ServeConn(conn)
	}
}

// ListenAndServe listens on the TCP network address addr and then calls
// Serve with handler to handle requests on incoming connections.
func ListenAndServe(addr string, handler Server) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	s := server{Handler: handler}
	s.Serve(listener)
}

// ListenAndServeTLS 作用和 ListenAndServe 一样，不过使用更安全的 TLS 连接
func ListenAndServeTLS(addr string, handler Server) {
	// http://c.biancheng.net/view/4530.html
	// https://colobu.com/2016/06/07/simple-golang-tls-examples/
	//pemCert, pemKey, _, err := GeneratePEM([]string{addr, "www.random.com"})
	//if err != nil {
	//	panic(fmt.Errorf("failed to generate PEM: %#v", err))
	//}
	//pemCert, pemKey, rootPEM := GeneratePEMWithRoot()

	pemCert, pemKey := GetCert(ServerCert)
	cert, err := tls.X509KeyPair(pemCert, pemKey)
	//cert, err := tls.LoadX509KeyPair("certs/server.pem", "certs/server.key")
	if err != nil {
		panic(fmt.Errorf("server: loadkeys: %s", err))
	}

	//clientCert, err := ioutil.ReadFile("certs/client.pem")
	//if err != nil {
	//	panic("Unable to read client.pem")
	//}
	clientCert, _ := GetCert(ClientCert)

	clientCertPool := x509.NewCertPool()
	if ok := clientCertPool.AppendCertsFromPEM(clientCert); !ok { // AppendCertsFromPEM(clientCert)
		panic("failed to parse root certificate")
	}

	config := tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCertPool,
		Time:         time.Now,
		Rand:         rand.Reader,
	}

	listener, err := tls.Listen("tcp", addr, &config)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	fmt.Printf("Listening %s %s\n", listener.Addr().Network(), listener.Addr().String())

	s := server{Handler: handler}
	s.Serve(listener)
}

// SendServer 发送服务
//
// 监听指定地址, 有客户端连接接入, 就给对方发送 PacketToSend
type SendServer struct {
	Sender
}

func NewSendServer(s Sender) *SendServer {
	return &SendServer{Sender: s}
}

// ServeConn 监听指定地址, 有客户端连接接入, 就给对方发送 PacketToSend
func (s SendServer) ServeConn(conn net.Conn) {
	fmt.Println("SendServer: send to", conn.RemoteAddr().String())
	s.Send(conn)
}

// ReceiveServer 接收服务
//
// 监听指定地址, 等待客户端连接接入, 接收对方发来的 Packet, 解析并处理接收到的 Packet。
type ReceiveServer struct {
	*Receiver
}

func NewReceiveServer() *ReceiveServer {
	return &ReceiveServer{Receiver: NewReceiver()}
}

// ServeConn 监听指定地址, 等待客户端连接接入,
// 接收对方发来的 Packet, 交给 HandlePacket 处理
func (r ReceiveServer) ServeConn(conn net.Conn) {
	fmt.Println("ReceiveServer: connect", conn.RemoteAddr().String())
	r.ReceiveAndHandle(conn)
}
