package jtrpc

import (
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/rpc"
	"os"
)

func RpcServer() {

	log.SetLevel(log.FatalLevel)

	args := os.Args

	rpcJitu := new(RPCJitu)
	rpc.Register(rpcJitu)
	rpc.HandleHTTP()
	address := ":1234"
	if len(args) > 1 {
		address = ":" + args[1]
	}
	l, e := net.Listen("tcp", address)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
	log.Infof("RPC Server start %v", address)

	select {}
}
