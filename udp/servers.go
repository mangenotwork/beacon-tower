package udp

import (
	"fmt"
	"net"
)

type Servers struct {
	Addr string
	Port int
}

func NewServers(addr string, port int) {
	s := Servers{
		Addr: addr,
		Port: port,
	}
	s.Run()
}

func (s *Servers) Run() {
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(s.Addr), Port: s.Port})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("run udp server: <%s> \n", listener.LocalAddr().String())

	// 读取数据
	data := make([]byte, 1024)
	for {
		n, remoteAddr, err := listener.ReadFromUDP(data)
		if err != nil {
			fmt.Printf("error during read: %s", err)
		}
		fmt.Printf("<%s> %s\n", remoteAddr, data[:n])
		_, err = listener.WriteToUDP([]byte("world"), remoteAddr)
		if err != nil {
			fmt.Printf(err.Error())
		}
	}
}
