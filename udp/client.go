package udp

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	ServersHost string // serversIP:port
}

func NewClient(host string) {
	c := Client{
		ServersHost: host,
	}
	c.Run()
}

func (c *Client) Run() {
	sHost := strings.Split(c.ServersHost, ":")
	sip := net.ParseIP(sHost[0])
	sport, err := strconv.Atoi(sHost[1])
	srcAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	dstAddr := &net.UDPAddr{IP: sip, Port: sport}
	conn, err := net.DialUDP("udp", srcAddr, dstAddr)
	if err != nil {
		fmt.Println(err)
	}
	defer conn.Close()

	go func() {
		data := make([]byte, 1024)
		for {
			n, remoteAddr, err := conn.ReadFromUDP(data)
			if err != nil {
				fmt.Printf("error during read: %s", err.Error())
			}
			fmt.Printf("<%s> %s\n", remoteAddr, data[:n])
		}
	}()

	for {
		time.Sleep(1 * time.Second)
		_, err := conn.Write([]byte("hello"))
		if err != nil {
			fmt.Printf("error write: %s", err.Error())
		}
		fmt.Printf("<%s>\n", conn.RemoteAddr())
	}
}
