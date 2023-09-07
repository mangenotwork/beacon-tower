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
	ConnectCode string // 连接code 是静态的由server端配发
	State       int    // 0:未连接   1:连接成功  2:server端丢失
	Sign        string // 签名
}

type ClientConf struct {
	ConnectCode string
}

func NewClient(host string, conf ...ClientConf) *Client {
	c := &Client{
		ServersHost: host,
	}
	if len(conf) >= 1 {
		if len(conf[0].ConnectCode) > 0 {
			c.ConnectCode = conf[0].ConnectCode
		}
	} else {
		c.DefaultConnectCode()
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

	// 启动一个携程用于与servers进行交互,外部不可操作
	go func() {
		data := make([]byte, 1024)
		for {
			n, remoteAddr, err := c.Conn.ReadFromUDP(data)
			if err != nil {
				fmt.Printf("error during read: %s", err.Error())
			}
			fmt.Printf("<%s> %s\n", remoteAddr, data[:n])
			fmt.Println("解包....size = ", n)
			packet := PacketDecrypt(data, n)
			switch packet.Command {
			case CommandReply:
				reply := &Reply{}
				err := DataDecoder(packet.Data, &reply)
				if err != nil {
					log.Println("返回的包解析失败， err = ", err)
				}
				log.Println(reply.Type, string(reply.Data))

				switch CommandCode(reply.Type) {
				case CommandConnect:
					log.Println("收到连接的反馈与下发的签名: ", string(reply.Data))
					// 存储签名
					c.Sign = string(reply.Data)
					c.State = 1
				case CommandPut:
					if c.Sign != packet.Sign {
						log.Println("未知主机认证!")
						return
					}
					ackState, err := bytesToInt(reply.Data)
					if err != nil {
						log.Println("解析ackState失败, err = ", err)
					}
					log.Println("ackState = ", ackState)

					if ackState == 0 {
						// 发送成功
					}

					if ackState == 1 {
						// 发送失败
					}

				}

			}
		}
	}()

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
	if c.State != 1 {
		log.Println("还未认证连接")
		return
	}
	data, err := PacketEncoder(CommandPut, "client", c.Sign, data)
	if err != nil {
		fmt.Println(err)
	}
	log.Println(len(data), string(data))
	c.Write(data)
}

// ConnectServers 请求连接服务器，获取签名
// 内容是发送 Connect code
func (c *Client) ConnectServers() {
	data, err := PacketEncoder(CommandConnect, c.Name, c.Sign, []byte(c.ConnectCode))
	if err != nil {
		fmt.Println(err)
	}
	c.Write(data)

}

func (c *Client) DefaultConnectCode() {
	c.ConnectCode = DefaultConnectCode
}
