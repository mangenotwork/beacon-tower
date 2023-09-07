package udp

import (
	"fmt"
	"log"
	"net"
)

type Servers struct {
	Addr        string
	Port        int
	Conn        *net.UDPConn
	Name        string                 // servers端的名称
	CMap        map[string]*ClientAddr // 存放客户端连接信息
	ConnectCode string                 // 连接code 是静态的由server端配发
}

type ServersConf struct {
	Name        string // servers端的名称
	ConnectCode string // 连接code 是静态的由server端配发
}

func NewServers(addr string, port int, conf ...ServersConf) (*Servers, error) {
	var err error
	s := &Servers{
		Addr: addr,
		Port: port,
	}
	if len(conf) >= 1 {
		if len(conf[0].Name) > 0 && len(conf[0].Name) <= 7 {
			s.Name = conf[0].Name
		}
		if len(conf[0].Name) > 7 {
			return nil, fmt.Errorf("启动失败，服务器命名不能超过7个字符")
		}
		if len(conf[0].ConnectCode) > 0 {
			s.ConnectCode = conf[0].ConnectCode
		}
	} else {
		s.DefaultServersName()
		s.DefaultConnectCode()
	}
	s.Conn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(s.Addr), Port: s.Port})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	fmt.Printf("udp server 启动成功 -->  name:%s |  addr: %s  | conn_code: %s \n", s.Name, s.Conn.LocalAddr().String(), s.ConnectCode)
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
		fmt.Println("解包....size = ", n)
		packet := PacketDecrypt(data, n)
		go func() {
			switch packet.Command {
			case CommandConnect:
				log.Println("请求连接...")
				if string(packet.Data) != s.ConnectCode {
					log.Println("未知客户端，签名不匹配...")
					return
				}
				// 下发签名
				s.ReplyConnect(remoteAddr)

			case CommandPut:
				log.Println("接收发送来的数据...")
				// 验证签名
				if !SignCheck(remoteAddr.IP.String(), packet.Sign) {
					log.Println("签名失败...")
					s.ReplyPut(remoteAddr, 1)
				} else {
					log.Println("签名成功...")
					log.Println("收到数据: ", string(packet.Data))
					s.ReplyPut(remoteAddr, 0)
				}

			case CommandHeartbeat:
				log.Println("接收到心跳包...")

			default:
				// 未知包丢弃
				log.Println("未知包!!!")
				return
			}
		}()
		//f(remoteAddr, data)
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

func (s *Servers) NewSign() string {
	return randStringBytes(7)
}

type Reply struct {
	Type int
	Data []byte
}

func (s *Servers) ReplyConnect(client *net.UDPAddr) {
	sign := s.NewSign()
	log.Println("生成签名 : ", sign)
	reply := &Reply{
		Type: int(CommandConnect),
		Data: []byte(sign),
	}
	b, e := DataEncoder(reply)
	if e != nil {
		log.Println("打包数据失败, e= ", e)
	}
	data, err := PacketEncoder(CommandReply, s.Name, sign, b)
	if err != nil {
		fmt.Println(err)
	}
	// 存储这个 sign  ip:sign
	SignStore(client.IP.String(), sign)
	s.Write(client, data)
}

// ReplyPut  响应put  state:0x0 成功   state:0x1 签名失败
func (s *Servers) ReplyPut(client *net.UDPAddr, state int) {
	stateB, _ := intToBytes(state)
	reply := &Reply{
		Type: int(CommandPut),
		Data: stateB,
	}
	b, e := DataEncoder(reply)
	if e != nil {
		log.Println("打包数据失败, e= ", e)
	}
	sign := SignGet(client.IP.String())
	data, err := PacketEncoder(CommandReply, s.Name, sign, b)
	if err != nil {
		fmt.Println(err)
	}
	s.Write(client, data)
}

func (s *Servers) DefaultServersName() {
	s.Name = DefaultServersName
}

func (s *Servers) DefaultConnectCode() {
	s.ConnectCode = DefaultConnectCode
}

// SetConnectCode 设置连接code,调用方自定义内容
func (s *Servers) SetConnectCode(code string) {
	s.ConnectCode = code
}

type ClientAddr struct {
	Connect []*ClientConnectObj
}

type ClientConnectObj struct {
	IP   string
	Addr *net.UDPAddr
}
