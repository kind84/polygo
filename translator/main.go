package main

import (
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/kind84/polygo/translator/translator"
)

func startServer() {
	t := new(translator.Translator)

	server := rpc.NewServer()
	server.Register(t)

	l, err := net.Listen("tcp", ":8090")
	if err != nil {
		log.Fatalln("listen error:", err)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalln(err)
		}

		go server.ServeCodec(jsonrpc.NewServerCodec(conn))
	}
}

func main() {
	log.Println("Listening on port 8090")
	startServer()
}
