package udp

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

type Client struct {
	ServersHost string // serversIP:port
	Conn        *net.UDPConn
	Name        string // client的名称
	ConnectCode string // 连接code
}

func NewClient(host string) *Client {
	c := &Client{
		ServersHost: host,
	}
	sHost := strings.Split(c.ServersHost, ":")
	sip := net.ParseIP(sHost[0])
	sport, err := strconv.Atoi(sHost[1])
	srcAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	dstAddr := &net.UDPAddr{IP: sip, Port: sport}
	c.Conn, err = net.DialUDP("udp", srcAddr, dstAddr)
	if err != nil {
		fmt.Println(err)
	}
	return c
}

func (c *Client) Close() {
	if c.Conn == nil {
		return
	}
	err := c.Conn.Close()
	if err != nil {
		fmt.Printf(err.Error())
	}
}

func (c *Client) Read(f func(data []byte)) {
	go func() {
		data := make([]byte, 1024)
		for {
			n, remoteAddr, err := c.Conn.ReadFromUDP(data)
			if err != nil {
				fmt.Printf("error during read: %s", err.Error())
			}
			fmt.Printf("<%s> %s\n", remoteAddr, data[:n])
			fmt.Println("解包....")
			PacketDecrypt(data, n)
			f(data)
		}
	}()
}

func (c *Client) Write(data []byte) {
	_, err := c.Conn.Write(data)
	if err != nil {
		fmt.Printf("error write: %s", err.Error())
	}
	fmt.Printf("<%s>\n", c.Conn.RemoteAddr())
}

func (c *Client) Put(data []byte) {
	data, err := PacketEncoder(CommandPut, "client", "", data)
	if err != nil {
		fmt.Println(err)
	}
	log.Println(len(data), string(data))
	c.Write(data)
}

// ConnectServers 请求连接服务器，获取签名
// 内容是发送 Connect code
func (c *Client) ConnectServers() {
	data, err := PacketEncoder(CommandConnect, "client", "", []byte("aaaaa"))
	if err != nil {
		fmt.Println(err)
	}
	log.Println(len(data), string(data))
	c.Write(data)
}
