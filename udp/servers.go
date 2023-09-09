package udp

import (
	"fmt"
	"github.com/mangenotwork/common/log"
	"net"
	"sync"
	"time"
)

type Servers struct {
	Addr        string
	Port        int
	Conn        *net.UDPConn
	name        string                         // servers端的名称
	CMap        map[string][]*ClientConnectObj // 存放客户端连接信息
	connectCode string                         // 连接code 是静态的由server端配发
	secretKey   string                         // 数据传输加密解密秘钥
	PutHandle   ServersPutFunc
	GetHandle   ServersGetFunc
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
		CMap:      make(map[string][]*ClientConnectObj),
		PutHandle: make(ServersPutFunc),
		GetHandle: make(ServersGetFunc),
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

	// 启动一个时间轮维护c端的连接
	s.timeWheel()

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
			case CommandConnect, CommandHeartbeat: // 自行维护，外部不可改变
				log.Info("请求连接或心跳包...")
				if string(packet.Data) != s.connectCode {
					log.Info("未知客户端，连接code不正确...")
					return
				}

				// 存储c端的连接
				s.ClientJoin(packet.Name, remoteAddr.IP.String(), remoteAddr)

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

			case CommandGet:
				log.Info("接收Get请求...")
				// 验证签名
				if !SignCheck(remoteAddr.String(), packet.Sign) {
					log.Info("签名失败...")
					s.ReplyPut(remoteAddr, 0, 1)
				} else {
					log.Info("签名成功...")
					//log.Info("收到数据: ", string(packet.Data))

					getData := &GetData{}
					err := ByteToObj(packet.Data, &getData)
					if err != nil {
						log.Error("解析put err :", err)
					}
					log.Info("putData.Label -> ", getData.Label)
					if fn, ok := s.GetHandle[getData.Label]; ok {
						code, rse := fn(s, getData.Param)
						getData.Response = rse
						gb, err := ObjToByte(getData)
						if err != nil {
							log.Error("对象转字节错误...")
						}
						s.ReplyGet(remoteAddr, getData.Id, code, gb)
					}

				}

			case CommandNotice: // 收到客户端通知的响应
				log.Info("收到客户端通知的响应...")
				notice := &NoticeData{}
				err := ByteToObj(packet.Data, &notice)
				if err != nil {
					log.Error("返回的包解析失败， err = ", err)
				}
				if v, ok := NoticeDataMap.Load(notice.Id); ok {
					v.(*NoticeData).ctxChan <- true
				}

			case CommandReply: // 收到客户端的回复
				log.Info("client端的回复...")
				reply := &Reply{}
				err := ByteToObj(packet.Data, &reply)
				if err != nil {
					log.Error("返回的包解析失败， err = ", err)
				}
				log.Info("收到包 id: ", reply.Type)
				switch CommandCode(reply.Type) {
				case CommandGet:
					log.Info("请求 ID: %d | StateCode: %d", reply.CtxId, reply.StateCode)
					getData := &GetData{}
					err := ByteToObj(reply.Data, &getData)
					if err != nil {
						log.Error("解析put err :", err)
					}
					log.Info("getData.Label -> ", getData.Label)
					getF, _ := GetDataMap.Load(getData.Id)
					getF.(*GetData).Response = getData.Response
					getF.(*GetData).ctxChan <- true
				}

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

func (s *Servers) Get(funcLabel, name string, param []byte) ([]byte, error) {
	if name == "" {
		name = formatName(DefaultClientName)
	}
	ip, ok := s.GetClientConn(name)
	if !ok && len(ip) < 1 {
		return nil, fmt.Errorf("客户端连接不存在")
	}
	return s.get(1000, funcLabel, name, ip[0].IP, param)
}

func (s *Servers) GetAtNumber(funcLabel, name string, param []byte, number int) ([]byte, error) {
	if name == "" {
		name = formatName(DefaultClientName)
	}
	ip, ok := s.GetClientConn(name)
	if !ok && len(ip) < number {
		return nil, fmt.Errorf("客户端连接不存在")
	}
	return s.get(1000, funcLabel, name, ip[number].IP, param)
}

func (s *Servers) GetAtIP(funcLabel, name, ip string, param []byte) ([]byte, error) {
	return s.get(1000, funcLabel, name, ip, param)
}

// Get  向指定 client获取数据，  针对name,ip, 获取指定name或ip Client的数据
// 指定一个超时时间
func (s *Servers) get(timeOut int, funcLabel, name, ip string, param []byte) ([]byte, error) {
	getData := &GetData{
		Label:    funcLabel,
		Id:       id(),
		Param:    param,
		ctxChan:  make(chan bool),
		Response: make([]byte, 0),
	}
	GetDataMap.Store(getData.Id, getData)
	b, err := ObjToByte(getData)
	if err != nil {
		log.Error("ObjToByte err = ", err)
	}

	c, ok := s.GetClientConnFromIP(name, ip)
	if !ok {
		return nil, fmt.Errorf("客户端连接不存在")
	}
	sign := SignGet(c.String())
	packet, err := PacketEncoder(CommandGet, s.name, sign, s.secretKey, b)
	if err != nil {
		log.Error(err)
	}
	s.Write(c, packet)
	select {
	case <-getData.ctxChan:
		log.Info("收到get返回的数据...")
		res := getData.Response
		GetDataMap.Delete(getData.Id)
		return res, nil
	case <-time.After(time.Millisecond * time.Duration(timeOut)):
		GetDataMap.Delete(getData.Id)
		return nil, fmt.Errorf("请求超时...")
	}
}

// Notice  通知方法:针对 name,对Client发送通知
// 特点: 1. 重试次数 2. 指定时间内重试
func (s *Servers) Notice(name, label string, data []byte) (string, error) {
	log.Info("发送通知消息......")
	if name == "" {
		name = formatName(DefaultClientName)
	}
	// 直接下发消息，等待c端应答
	client, ok := s.GetClientConn(name)
	log.Info("client = ", name, client, ok)
	if !ok {
		return "未找到客户端", fmt.Errorf("未找到客户端...")
	}
	// 组建通知包
	packetMap := make(map[*net.UDPAddr]*NoticeData, 0)
	for _, c := range client {
		noticeData := &NoticeData{
			Label:   label,
			Id:      id(),
			Data:    data,
			ctxChan: make(chan bool),
		}
		NoticeDataMap.Store(noticeData.Id, noticeData)
		packetMap[c.Addr] = noticeData
		go func() {
			for {
				timer := time.NewTimer(10 * time.Second)
				select {
				case <-noticeData.ctxChan:
					log.Info("收到client通知反馈... id = ", noticeData.Id)
					NoticeDataMap.Delete(noticeData.Id)
					return
				case <-timer.C: // 超过10秒，表示已经大于最大重试的时间，释放内存
					NoticeDataMap.Delete(noticeData.Id)
					log.Info("超过10秒，表示已经大于最大重试的时间，释放内存")
					return
				}
			}

		}()
	}
	if s.noticeSend(packetMap) {
		return "通知下发完成", nil
	}
	retry := 1    // 重试次数
	retryMax := 3 // 最大重试3次
	for {
		if retry > retryMax {
			// TODO 找到是哪个节点未收到通知
			return "重试次数完，还有客户端未收到通知", fmt.Errorf("重试次数完，还有客户端未收到通知")
		}
		timer := time.NewTimer(3 * time.Second) // 3秒重试一次
		select {
		case <-timer.C:
			// 检查通知
			if s.noticeSend(packetMap) {
				return "通知下发完成", nil
			}
			retry++
		}
	}

}

func (s *Servers) noticeSend(packetMap map[*net.UDPAddr]*NoticeData) bool {
	finish := true
	for cConn, v := range packetMap {
		_, has := NoticeDataMap.Load(v.Id)
		log.Info("发送消息id = ", v.Id, "  -> has = ", has)
		if has {
			finish = false
			b, err := ObjToByte(v)
			if err != nil {
				log.Error("ObjToByte err = ", err)
			}
			sign := SignGet(cConn.String())
			packet, err := PacketEncoder(CommandNotice, s.name, sign, s.secretKey, b)
			if err != nil {
				log.Error(err)
			}
			s.Write(cConn, packet)
		}
	}
	return finish
}

type Reply struct {
	Type      int
	CtxId     int64 // 数据包上下文的交互id
	Data      []byte
	StateCode int // 状态码  0:成功  1:认证失败  2:自定义错误
}

func (s *Servers) ReplyConnect(client *net.UDPAddr) {
	sign := createSign()
	log.Info("生成签名 : ", sign)
	reply := &Reply{
		Type:      int(CommandConnect),
		Data:      []byte(sign),
		CtxId:     0,
		StateCode: 0,
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
		Type:      int(CommandPut),
		CtxId:     id,
		Data:      stateB,
		StateCode: int(state),
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

// ReplyGet 返回put  state:0x0 成功   state:0x1 签名失败  state:2 业务层面的失败
func (s *Servers) ReplyGet(client *net.UDPAddr, id int64, state int, data []byte) {
	reply := &Reply{
		Type:      int(CommandGet),
		CtxId:     id,
		Data:      data,
		StateCode: state,
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

func (s *Servers) GetHandleFunc(label string, f func(s *Servers, param []byte) (int, []byte)) {
	s.GetHandle[label] = f
}

func (s *Servers) GetServersName() string {
	return s.name
}

func (s *Servers) ClientJoin(name, ip string, addr *net.UDPAddr) {
	client := &ClientConnectObj{
		IP:   ip,
		Addr: addr,
		Last: time.Now().Unix(),
	}
	isHas := false
	if v, ok := s.CMap[name]; ok {
		for _, c := range v {
			if c.Addr.String() == addr.String() {
				isHas = true
				c.Addr = addr
				c.Last = client.Last
				break
			}
		}
	}
	if isHas {
		return
	}
	s.CMap[name] = append(s.CMap[name], client)
	return
}

func (s *Servers) ClientDiscard(name, ip string) {
	log.Info("删除离线的c")
	if v, ok := s.CMap[name]; ok {
		for i, c := range v {
			log.Info(i, c.IP, ip)
			if c.IP == ip {
				v = append(v[:i], v[i+1:]...)
			}
		}
		s.CMap[name] = v
	}
}

func (s *Servers) GetClientConn(name string) ([]*ClientConnectObj, bool) {
	if v, ok := s.CMap[name]; ok {
		return v, true
	}
	return nil, false
}

func (s *Servers) GetClientConnFromIP(name, ip string) (*net.UDPAddr, bool) {
	if list, ok := s.GetClientConn(name); ok {
		for _, c := range list {
			if c.IP == ip {
				return c.Addr, true
			}
		}
	}
	return nil, false
}

func (s *Servers) timeWheel() {
	go func() {
		tTime := time.Duration(2)
		for {
			// 6s检查一次连接
			timer := time.NewTimer(tTime * time.Second)
			select {
			case <-timer.C:
				//log.Info("时间轮检查c端的连接...")
				t := time.Now().Unix()
				for k, v := range s.CMap {
					for _, c := range v {
						if t-c.Last > 6 { // 这个时间要大于5秒，因为来自c端的心跳就是5秒
							//log.InfoF("离线服务器名称:%s IP地址:%s  当前t=%d last=%d", k, c.IP, t, c.Last)
							s.ClientDiscard(k, c.IP)
						} else {
							//log.InfoF("在线服务器名称:%s IP地址:%s  当前t=%d last=%d", k, c.IP, t, c.Last)
						}
					}
				}
			}
		}
	}()
}

type ClientConnectObj struct {
	IP   string
	Addr *net.UDPAddr
	Last int64 // 最后一次连接的时间
}

// TODO ... 要定下来确定ip下的节点离线，还是name下的任意节点离线?
var HeartbeatTable sync.Map

func HeartbeatTableShow() {
	log.Info("+++++++++++++++++++++++++++")
	t := time.Now().Unix()
	HeartbeatTable.Range(func(key, value any) bool {
		log.Info(key, value, t)
		if t-value.(int64) > 5 {
			log.InfoF("node:%s 离线", key)
		} else {
			log.InfoF("node:%s 在线 %d", key, value)
		}
		return true
	})
	log.Info("+++++++++++++++++++++++++++")
}
