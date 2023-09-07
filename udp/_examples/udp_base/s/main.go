package main

import (
	"beacon-tower/udp"
	"log"
)

func main() {
	s, e := udp.NewServers("0.0.0.0", 12345)
	if e != nil {
		panic(e)
	}
	s.PutHandleFunc("case1", Case1)
	s.PutHandleFunc("case2", Case2)
	s.Run()
}

func Case1(s *udp.Servers, body []byte) {
	log.Println("Case1 func --> ", string(body))
}

func Case2(s *udp.Servers, body []byte) {
	log.Println("Case2 func --> ", string(body))
	log.Println("servers name = ", s.GetServersName())
}
