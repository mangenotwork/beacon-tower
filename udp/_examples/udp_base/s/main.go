package main

import (
	"beacon-tower/udp"
	"log"
	"os"
	"time"
)

var testFile = "test.txt"

func init() {}

func main() {
	s, e := udp.NewServers("0.0.0.0", 12345)
	if e != nil {
		panic(e)
	}
	s.PutHandleFunc("case1", Case1)
	s.PutHandleFunc("case2", Case2)

	go func() {
		for {
			time.Sleep(2 * time.Second)
			udp.HeartbeatTableShow()
		}
	}()

	s.Run()
}

func Case1(s *udp.Servers, body []byte) {
	log.Println("Case1 func --> ")
}

func Case2(s *udp.Servers, body []byte) {
	log.Println("Case2 func --> ", string(body))
	log.Println("servers name = ", s.GetServersName())
	file, err := os.OpenFile(testFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	content := []byte(string(body) + "\n")
	_, err = file.Write(content)
	if err != nil {
		panic(err)
	}
}
