package udp

import (
	"github.com/mangenotwork/common/log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
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
		state:       0,
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
				case CommandConnect: // 连接包与心跳包的反馈会触发
					log.Info("收到连接的反馈与下发的签名: ", string(reply.Data))
					// 存储签名
					c.sign = string(reply.Data)
					c.state = 1
					// 将积压的数据进行发送
					c.SendBacklog()
				case CommandPut:
					if c.sign != packet.Sign {
						log.Info("未知主机认证!")
						return
					}
					res, err := bytesToInt64(reply.Data)
					if err != nil {
						log.Error("解析ackState失败, err = ", err)
					}

					if res == 1 {
						// 签名错误
						log.Error("签名错误")
						c.ConnectServers()
						break
					}

					// 服务端以确认收到删除对应的数据
					log.Info("putId = ", reply.PutId)
					backlogDel(reply.PutId)
				}

			}
		}
	}()

	// 时间轮,心跳维护，动态刷新签名
	c.timeWheel()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		select {
		case s := <-ch:
			log.Info("通知退出....")
			toUdb() // 将积压的数据持久化
			if i, ok := s.(syscall.Signal); ok {
				os.Exit(int(i))
			} else {
				os.Exit(0)
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
			_, _ = PacketDecrypt(c.secretKey, data, n)
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

	putData := PutData{
		Label: funcLabel,
		Id:    id(),
		Body:  data,
	}
	// 数据被积压，占时保存
	backlogAdd(putData.Id, putData)
	// 未与servers端确认连接，不发送数据
	if c.state != 1 {
		log.Info("还未认证连接")
		return
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

// 时间轮，持续制定时间发送心跳包
func (c *Client) timeWheel() {
	go func() {
		tTime := time.Duration(5) // 时间轮5秒
		for {
			// 5s维护一个心跳，s端收到心跳会返回新的签名
			timer := time.NewTimer(tTime * time.Second)
			select {
			case t := <-timer.C:
				// 这个时候表示连接不存在
				c.state = 0
				log.Info("发送心跳...", t)
				data, err := PacketEncoder(CommandHeartbeat, c.name, c.sign, c.secretKey, []byte(c.connectCode))
				if err != nil {
					log.Error(err)
				}
				c.Write(data)
			}
		}
	}()
}

// SendBacklog 发送积压的数据，
func (c *Client) SendBacklog() {
	backlog.Range(func(key, value any) bool {
		log.Info("重发 key = ", key)
		b, err := ObjToByte(value.(PutData))
		if err != nil {
			log.Error("ObjToByte err = ", err)
		}
		packet, err := PacketEncoder(CommandPut, c.name, c.sign, c.secretKey, b)
		if err != nil {
			log.Error(err)
		}
		c.Write(packet)
		return true
	})
	// 如果存在持久化积压数据则进行发送
	BacklogLoad()
}
