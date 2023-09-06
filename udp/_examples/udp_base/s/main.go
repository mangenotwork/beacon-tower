package main

import (
	"beacon-tower/udp"
	"log"
	"net"
)

func main() {
	s, e := udp.NewServers("0.0.0.0", 12345)
	if e != nil {
		panic(e)
	}
	s.Read(func(client *net.UDPAddr, data []byte) {
		log.Println("读取到数据：", string(data))
		s.Write(client, []byte("world."))
	})
}
