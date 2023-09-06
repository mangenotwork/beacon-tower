package main

import (
	"beacon-tower/udp"
	"time"
)

func main() {
	client := udp.NewClient("127.0.0.1:12345")
	//client.Read(func(data []byte) {
	//	log.Println("读取到的数据: ", string(data))
	//})
	for {
		time.Sleep(1 * time.Second)
		client.Put([]byte("hello"))
	}
}
