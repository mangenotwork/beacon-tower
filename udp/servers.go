package udp

import (
	"github.com/mangenotwork/common/log"
	"net"
)

type Servers struct {
	Addr        string
	Port        int
	Conn        *net.UDPConn
	name        string                 // servers端的名称
	CMap        map[string]*ClientAddr // 存放客户端连接信息
	connectCode string                 // 连接code 是静态的由server端配发
	secretKey   string                 // 数据传输加密解密秘钥
	PutHandle   ServersPutFunc
}

type ServersConf struct {
	Name        string // servers端的名称
	ConnectCode string // 连接code 是静态的由server端配发
	SecretKey   string // 数据传输加密解密秘钥
}

func NewServers(addr string, port int, conf ...ServersConf) (*Servers, error) {
	var err error
	s := &Servers{
		Addr:      addr,
		Port:      port,
		CMap:      map[string]*ClientAddr{},
		PutHandle: make(ServersPutFunc),
	}
	if len(conf) >= 1 {
		if len(conf[0].Name) > 0 && len(conf[0].Name) <= 7 {
			s.name = conf[0].Name
		}
		if len(conf[0].Name) > 7 {
			return nil, ErrNmeLengthAbove
		}
		if len(conf[0].ConnectCode) > 0 {
			s.connectCode = conf[0].ConnectCode
		}
	} else {
		s.DefaultServersName()
		s.DefaultConnectCode()
		s.DefaultSecretKey()
	}
	s.Conn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(s.Addr), Port: s.Port})
	if err != nil {
		log.Error(err)
		return nil, err
	}
	log.InfoF("udp server 启动成功 -->  name:%s |  addr: %s  | conn_code: %s \n",
		s.name, s.Conn.LocalAddr().String(), s.connectCode)
	return s, nil
}

func (s *Servers) Run() {
	// 读取数据s
	data := make([]byte, 1024)
	for {
		n, remoteAddr, err := s.Conn.ReadFromUDP(data)
		if err != nil {
			log.ErrorF("error during read: %s", err)
		}
		//log.InfoF("<%s> %s\n", remoteAddr, data[:n])
		log.Info("解包....size = ", n)
		packet, err := PacketDecrypt(s.secretKey, data, n)
		if err != nil {
			log.Error("错误的包 err:", err)
			continue
		}
		go func() {
			switch packet.Command {
			case CommandConnect: // 自行维护，外部不可改变
				log.Info("请求连接...")
				if string(packet.Data) != s.connectCode {
					log.Info("未知客户端，连接code不正确...")
					return
				}
				// 下发签名
				s.ReplyConnect(remoteAddr)

			case CommandPut: // 提供外部接口，供外部使用
				log.Info("接收发送来的数据...")
				// 验证签名
				if !SignCheck(remoteAddr.String(), packet.Sign) {
					log.Info("签名失败...")
					s.ReplyPut(remoteAddr, 0, 1)
				} else {
					log.Info("签名成功...")
					//log.Info("收到数据: ", string(packet.Data))

					putData := &PutData{}
					err := ByteToObj(packet.Data, &putData)
					if err != nil {
						log.Error("解析put err :", err)
					}
					log.Info("putData.Label -> ", putData.Label)
					if fn, ok := s.PutHandle[putData.Label]; ok {
						fn(s, putData.Body)
					}

					s.ReplyPut(remoteAddr, putData.Id, 0)
				}

			case CommandHeartbeat: // 自行维护外部不可改变
				log.Info("接收到心跳包...")
				if string(packet.Data) != s.connectCode {
					log.Info("未知客户端，连接code不正确...")
					return
				}
				// 下发签名
				s.ReplyConnect(remoteAddr)

			default:
				// 未知包丢弃
				log.Info("未知包!!!")
				return
			}
		}()
		//f(remoteAddr, data)
	}
}

func (s *Servers) Write(client *net.UDPAddr, data []byte) {
	_, err := s.Conn.WriteToUDP(data, client)
	if err != nil {
		log.Error(err.Error())
	}
}

func (s *Servers) Put(client *net.UDPAddr, data []byte) {
	sign := SignGet(client.String())
	data, err := PacketEncoder(CommandPut, s.name, sign, s.secretKey, data)
	if err != nil {
		log.Error(err)
	}
	s.Write(client, data)
}

type Reply struct {
	Type  int
	PutId int64
	Data  []byte
}

func (s *Servers) ReplyConnect(client *net.UDPAddr) {
	sign := createSign()
	log.Info("生成签名 : ", sign)
	reply := &Reply{
		Type: int(CommandConnect),
		Data: []byte(sign),
	}
	b, e := ObjToByte(reply)
	if e != nil {
		log.Error(" e= ", e)
	}
	data, err := PacketEncoder(CommandReply, s.name, sign, s.secretKey, b)
	if err != nil {
		log.Error(err)
	}
	// 存储这个 sign  ip+port:sign
	SignStore(client.String(), sign)
	s.Write(client, data)
}

// ReplyPut  响应put  state:0x0 成功   state:0x1 签名失败
func (s *Servers) ReplyPut(client *net.UDPAddr, id, state int64) {
	stateB, _ := int64ToBytes(state)
	reply := &Reply{
		Type:  int(CommandPut),
		PutId: id,
		Data:  stateB,
	}
	b, e := ObjToByte(reply)
	if e != nil {
		log.Error("打包数据失败, e= ", e)
	}
	sign := SignGet(client.String())
	data, err := PacketEncoder(CommandReply, s.name, sign, s.secretKey, b)
	if err != nil {
		log.Error(err)
	}
	s.Write(client, data)
}

func (s *Servers) DefaultServersName() {
	s.name = DefaultServersName
}

func (s *Servers) DefaultConnectCode() {
	s.connectCode = DefaultConnectCode
}

func (s *Servers) DefaultSecretKey() {
	s.secretKey = DefaultSecretKey
}

// SetConnectCode 设置连接code,调用方自定义内容
func (s *Servers) SetConnectCode(code string) {
	s.connectCode = code
}

func (s *Servers) PutHandleFunc(label string, f func(s *Servers, body []byte)) {
	s.PutHandle[label] = f
}

func (s *Servers) GetServersName() string {
	return s.name
}

type ClientAddr struct {
	Connect []*ClientConnectObj
}

type ClientConnectObj struct {
	IP   string
	Addr *net.UDPAddr
}
