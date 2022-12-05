package jtrpc

import (
	log "github.com/sirupsen/logrus"
	"net/rpc"
	"testing"
	"time"
)

func TestPRCClient(t *testing.T) {
	serverAddress := "localhost:1234"
	serverAddress = "jt.cupb.top:5321"
	client, err := rpc.DialHTTP("tcp", serverAddress)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	var reply Response

	r := Request{
		Uuid:       time.Now().String(),
		HttpMethod: "GET",
		Url:        "https://jmsgw.jtexpress.com.cn/operatingplatform/order/getOrderDetail",
		Token:      "aaa11111111111111a",
		ParamBody:  "{1}",
	}

	for i := 0; i < 3; i++ {
		//go func() {
		log.Infof("%v", i)
		err = client.Call("RPCJitu.Request", r, &reply)
		if err != nil {
			log.Fatal("arith error:", err)
		} else {
			log.Infof("reply %v", reply)
		}
		//}()
	}

}

// 异步调用方案
func TestPRCAsyncClient(t *testing.T) {
	serverAddress := "localhost"
	//serverAddress = "jt.cupb.top"
	client, err := rpc.DialHTTP("tcp", serverAddress+":1234")
	if err != nil {
		log.Fatal("dialing:", err)
	}
	var reply Response

	r := Request{
		NetName: "消息来了1",
	}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	back := client.Go("RPCJitu.PostTest", r, &reply, nil)

	select {
	case replyCall := <-back.Done:
		if err = replyCall.Error; err != nil {
			log.Error("Multiply error:", err)
		} else {
			log.Infof("reply %v", reply)
		}
	case <-ticker.C:
		log.Println("tick")
	}
	//if err != nil {
	//	log.Fatal("arith error:", err)
	//} else {
	//	log.Infof("reply %v", reply)
	//}
}
