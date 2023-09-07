package udp

import (
	"github.com/mangenotwork/common/log"
	"net"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	ServersHost string // serversIP:port
	Conn        *net.UDPConn
	name        string // client的名称
	connectCode string // 连接code 是静态的由server端配发
	state       int    // 0:未连接   1:连接成功  2:server端丢失
	sign        string // 签名
	secretKey   string // 数据传输加密解密秘钥
}

type ClientConf struct {
	Name        string
	ConnectCode string
	SecretKey   string // 数据传输加密解密秘钥
}

func NewClient(host string, conf ...ClientConf) *Client {
	c := &Client{
		ServersHost: host,
	}
	if len(conf) >= 1 {
		if len(conf[0].ConnectCode) > 0 {
			c.connectCode = conf[0].ConnectCode
		}
	} else {
		c.DefaultClientName()
		c.DefaultConnectCode()
		c.DefaultSecretKey()
	}
	sHost := strings.Split(c.ServersHost, ":")
	sip := net.ParseIP(sHost[0])
	sport, err := strconv.Atoi(sHost[1])
	srcAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	dstAddr := &net.UDPAddr{IP: sip, Port: sport}
	c.Conn, err = net.DialUDP("udp", srcAddr, dstAddr)
	if err != nil {
		log.Error(err)
	}

	// 启动一个携程用于与servers进行交互,外部不可操作
	go func() {
		data := make([]byte, 1024)
		for {
			n, remoteAddr, err := c.Conn.ReadFromUDP(data)
			if err != nil {
				log.InfoF("error during read: %s", err.Error())
				c.state = 0 // 连接有异常更新连接状态
				continue
			}
			log.InfoF("<%s> %s\n", remoteAddr, data[:n])
			log.Info("解包....size = ", n)
			packet, err := PacketDecrypt(c.secretKey, data, n)
			if err != nil {
				log.Error("错误的包 err:", err)
				continue
			}
			switch packet.Command {
			case CommandReply:
				reply := &Reply{}
				err := ByteToObj(packet.Data, &reply)
				if err != nil {
					log.Error("返回的包解析失败， err = ", err)
				}
				log.Info(reply.Type, string(reply.Data))

				switch CommandCode(reply.Type) {
				case CommandConnect:
					log.Info("收到连接的反馈与下发的签名: ", string(reply.Data))
					// 存储签名
					c.sign = string(reply.Data)
					c.state = 1
				case CommandPut:
					if c.sign != packet.Sign {
						log.Info("未知主机认证!")
						return
					}
					ackState, err := bytesToInt(reply.Data)
					if err != nil {
						log.Error("解析ackState失败, err = ", err)
					}
					log.Info("ackState = ", ackState)

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
		log.Error(err.Error())
	}
}

func (c *Client) Read(f func(data []byte)) {
	go func() {
		data := make([]byte, 1024)
		for {
			n, remoteAddr, err := c.Conn.ReadFromUDP(data)
			if err != nil {
				log.ErrorF("error during read: %s", err.Error())
			}
			log.InfoF("<%s> %s\n", remoteAddr, data[:n])
			log.Info("解包....")
			PacketDecrypt(c.secretKey, data, n)
			f(data)
		}
	}()
}

func (c *Client) Write(data []byte) {
	_, err := c.Conn.Write(data)
	if err != nil {
		log.ErrorF("error write: %s", err.Error())
	}
	log.InfoF("<%s>\n", c.Conn.RemoteAddr())
}

func (c *Client) Put(funcLabel string, data []byte) {
R:
	if c.state != 1 {
		log.Info("还未认证连接")
		c.ConnectServers()
		time.Sleep(100 * time.Millisecond)
		goto R
	}
	putData := PutData{
		Label: funcLabel,
		Body:  data,
	}
	b, err := ObjToByte(putData)
	if err != nil {
		log.Error("ObjToByte err = ", err)
	}
	packet, err := PacketEncoder(CommandPut, c.name, c.sign, c.secretKey, b)
	if err != nil {
		log.Error(err)
	}
	c.Write(packet)
}

// ConnectServers 请求连接服务器，获取签名
// 内容是发送 Connect code
func (c *Client) ConnectServers() {
	data, err := PacketEncoder(CommandConnect, c.name, c.sign, c.secretKey, []byte(c.connectCode))
	if err != nil {
		log.Error(err)
	}
	c.Write(data)
}

func (c *Client) DefaultClientName() {
	c.name = DefaultClientName
}

func (c *Client) DefaultConnectCode() {
	c.connectCode = DefaultConnectCode
}

func (c *Client) DefaultSecretKey() {
	c.secretKey = DefaultSecretKey
}
