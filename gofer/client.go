package gofer

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
)

// Client 是客户端接口
type Client interface {
	Do(conn net.Conn) chan bool // Do 完成对服务端的请求, 结束后往返回的通道扔值
}

// DialAndRunClient 连接服务器，完成 client 的 Do
func DialAndRunClient(serverAddress string, client Client) {
	conn, err := net.Dial("tcp", serverAddress)
	if err != nil {
		panic(err)
	}

	<-client.Do(conn)
}

// DialAndRunClientTLS 作用和 DialAndRunClient 一样，不过使用更安全的 TLS 连接
func DialAndRunClientTLS(serverAddress string, client Client) {
	//pemCert, pemKey, _, err := GeneratePEM([]string{serverAddress, "www.random.com"})
	//if err != nil {
	//	panic(fmt.Errorf("failed to generate PEM: %#v", err))
	//}
	//pemCert, pemKey, rootPEM := GeneratePEMWithRoot()

	pemCert, pemKey := GetCert(ClientCert)
	cert, err := tls.X509KeyPair(pemCert, pemKey)
	//cert, err := tls.LoadX509KeyPair("certs/client.pem", "certs/client.key")
	if err != nil {
		panic(fmt.Errorf("server: loadkeys: %s", err))
	}

	//pemCert, err := ioutil.ReadFile("certs/client.pem")
	//if err != nil {
	//	panic("Unable to read cert.pem")
	//}

	clientCertPool := x509.NewCertPool()
	if ok := clientCertPool.AppendCertsFromPEM(pemCert); !ok {
		panic("failed to parse root certificate")
	}

	conf := tls.Config{
		InsecureSkipVerify: true,
		RootCAs:            clientCertPool,
		Certificates:       []tls.Certificate{cert},
	}
	conn, err := tls.Dial("tcp", serverAddress, &conf)
	if err != nil {
		panic(err)
	}

	<-client.Do(conn)
}

// SendClient 是发送的客户端
type SendClient struct {
	Sender
}

func NewSendClient(s Sender) *SendClient {
	return &SendClient{Sender: s}
}

func (s SendClient) Do(conn net.Conn) chan bool {
	fmt.Println("SendClient: send to", conn.RemoteAddr().String())

	done := make(chan bool)

	go func() {
		s.Send(conn)
		done <- true
	}()

	return done
}

// ReceiveClient 是接收的客户端
// 接入某服务器, 接收对方发来的 Packet, 解析并处理接收到的 Packet。
type ReceiveClient struct {
	*Receiver
}

func NewReceiveClient() *ReceiveClient {
	return &ReceiveClient{Receiver: NewReceiver()}
}

func (r ReceiveClient) Do(conn net.Conn) chan bool {
	fmt.Println("ReceiveClient: connect", conn.RemoteAddr().String())
	return r.ReceiveAndHandle(conn)
}
