package udp

import (
	"fmt"
	"net"
)

type Servers struct {
	Addr string
	Port int
	Conn *net.UDPConn
	Name string                 // servers端的名称
	CMap map[string]*ClientAddr // 存放客户端连接信息
}

func NewServers(addr string, port int) (*Servers, error) {
	var err error
	s := &Servers{
		Addr: addr,
		Port: port,
	}
	s.Conn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(s.Addr), Port: s.Port})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	fmt.Printf("run udp server: <%s> \n", s.Conn.LocalAddr().String())
	return s, nil
}

func (s *Servers) Read(f func(client *net.UDPAddr, data []byte)) {
	// 读取数据
	data := make([]byte, 1024)
	for {
		n, remoteAddr, err := s.Conn.ReadFromUDP(data)
		if err != nil {
			fmt.Printf("error during read: %s", err)
		}
		fmt.Printf("<%s> %s\n", remoteAddr, data[:n])

		fmt.Println("解包.... n=", n)
		PacketDecrypt(data, n)

		f(remoteAddr, data)
	}
}

func (s *Servers) Write(client *net.UDPAddr, data []byte) {
	_, err := s.Conn.WriteToUDP(data, client)
	if err != nil {
		fmt.Printf(err.Error())
	}
}

func (s *Servers) Put(client *net.UDPAddr, data []byte) {
	data, err := PacketEncoder(CommandPut, "server", "", data)
	if err != nil {
		fmt.Println(err)
	}
	s.Write(client, data)
}

type ClientAddr struct {
	Connect []*ClientConnectObj
}

type ClientConnectObj struct {
	IP   string
	Addr *net.UDPAddr
}
