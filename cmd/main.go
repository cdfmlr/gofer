package main

import (
	"flag"
	"fmt"
	"github.com/cdfmlr/gofer/gofer"
	"os"
)

func usage() {
	_, _ = fmt.Fprintf(flag.CommandLine.Output(), "gofer <send|recv> [-f=FILE] [-m=MESSAGE [-i INFO]] <-s|-c>=ADDRESS\n")
	_, _ = fmt.Fprintf(flag.CommandLine.Output(), " send: send things\n recv: receive things.\n")
	flag.PrintDefaults()
}

// 命令行参数
var (
	message string
	msgInfo string
	file    string
	bigFile string
	serve   string
	client  string
)

func init() {
	flag.StringVar(&message, "m", "", "`MESSAGE` to send. (Only for <gofer send>)")
	flag.StringVar(&msgInfo, "i", "", "`INFO` of message to send. (use with <gofer send -m xxx>)")
	flag.StringVar(&file, "f", "", "path of `FILE` to send (Only for <gofer send>)")
	flag.StringVar(&bigFile, "bigfile", "", "path of `BiG_FILE` to send (Only for <gofer send>)")
	flag.StringVar(&serve, "s", "", "start a server at given `ADDRESS`")
	flag.StringVar(&client, "c", "", "run as a client, connect to a server at given `ADDRESS`")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	cmd := os.Args[1]

	// flag.Parse() parses the command-line flags from os.Args[1:]
	os.Args = os.Args[1:]
	flag.Parse()

	if (serve == "") == (client == "") { // both exist or neither
		usage()
		return
	}

	switch cmd {
	case "send":
		cmdSend()
	case "recv":
		cmdRecv()
	default:
		usage()
	}
}

func cmdSend() {
	var sender gofer.Sender

	switch {
	case message != "":
		sender = gofer.NewMessageSender(msgInfo, message)
	case file != "":
		sender = gofer.NewSimpleFileSender(file)
	case bigFile != "":
		bfSender := gofer.NewBigFileSender()
		bfSender.AppendFile(bigFile)
		sender = bfSender
	default:
		panic("neither message nor file")
	}

	switch {
	case serve != "":
		address := serve
		server := gofer.NewSendServer(sender)
		//gofer.ListenAndServe(address, server)
		gofer.ListenAndServeTLS(address, server)
	case client != "":
		address := client
		client := gofer.NewSendClient(sender)
		//gofer.DialAndRunClient(address, client)
		gofer.DialAndRunClientTLS(address, client)
	default:
		panic("neither serve nor client")
	}
}

func cmdRecv() {
	switch {
	case serve != "":
		address := serve
		server := gofer.NewReceiveServer()
		//gofer.ListenAndServe(address, server)
		gofer.ListenAndServeTLS(address, server)
	case client != "":
		address := client
		client := gofer.NewReceiveClient()
		//gofer.DialAndRunClient(address, client)
		gofer.DialAndRunClientTLS(address, client)
	default:
		panic("neither serve nor client")
	}
}
