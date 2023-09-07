package main

import (
	"beacon-tower/udp"
	"time"
)

func main() {
	client := udp.NewClient("127.0.0.1:12345")
	client.ConnectServers() // 连接服务器
	for {
		time.Sleep(1 * time.Second)
		client.Put([]byte("hello"))
	}

	select {}
}
