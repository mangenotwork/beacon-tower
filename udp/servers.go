package udp

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Servers struct {
	Addr        string                         // 地址 默认0.0.0.0
	Port        int                            // 端口
	Conn        *net.UDPConn                   // S端的UDP连接对象
	name        string                         // servers端的名称
	CMap        map[string][]*ClientConnectObj // 存放客户端连接信息
	connectCode string                         // 连接code 是静态的由server端配发
	secretKey   string                         // 数据传输加密解密秘钥
	PutHandle   ServersPutFunc                 // PUT类型方法
	GetHandle   ServersGetFunc                 // GET类型方法
	onLineTable map[string]bool                // c端的在线表 key= name+ip
}

type ServersConf struct {
	Name        string // servers端的名称
	ConnectCode string // 连接code 是静态的由server端配发
	SecretKey   string // 数据传输加密解密秘钥 8个字节
}

func SetServersConf(serversName, connectCode, secretKey string) ServersConf {
	return ServersConf{
		Name:        serversName,
		ConnectCode: connectCode,
		SecretKey:   secretKey,
	}
}

func NewServers(addr string, port int, conf ...ServersConf) (*Servers, error) {
	var err error
	if len(addr) < 1 {
		addr = "0.0.0.0"
	}
	s := &Servers{
		Addr:        addr,
		Port:        port,
		CMap:        make(map[string][]*ClientConnectObj),
		PutHandle:   make(ServersPutFunc),
		GetHandle:   make(ServersGetFunc),
		onLineTable: make(map[string]bool),
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
		if len(conf[0].SecretKey) != 8 {
			return nil, ErrServersSecretKey
		} else {
			s.secretKey = conf[0].SecretKey
		}
	} else {
		s.DefaultServersName()
		s.DefaultConnectCode()
		s.DefaultSecretKey()
	}
	s.Conn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(s.Addr), Port: s.Port})
	if err != nil {
		Error(err)
		return nil, err
	}
	InfoF("udp server 启动成功 -->  name:%s |  addr: %s  | conn_code: %s \n",
		s.name, s.Conn.LocalAddr().String(), s.connectCode)
	return s, nil
}

func (s *Servers) SetServersName(name string) error {
	if len(name) > 0 && len(name) <= 7 {
		s.name = name
		return nil
	}
	return ErrNmeLengthAbove
}

func (s *Servers) SetConnectCode(code string) {
	s.connectCode = code
}

func (s *Servers) SetSecretKey(key string) error {
	if len(key) != 8 {
		return ErrServersSecretKey
	}
	s.secretKey = key
	return nil
}

func (s *Servers) Run() {

	// 启动一个时间轮维护c端的连接
	s.timeWheel()

	data := make([]byte, 1500)
	for {
		n, remoteAddr, err := s.Conn.ReadFromUDP(data)
		if err != nil {
			ErrorF("error during read: %s", err)
		}
		Info("解包....size = ", n)
		packet, err := PacketDecrypt(s.secretKey, data, n)
		if err != nil {
			Error("错误的包 err:", err)
			continue
		}
		go func() {
			switch packet.Command {
			case CommandConnect, CommandHeartbeat:
				Info("请求连接或心跳包...")
				if string(packet.Data) != s.connectCode {
					Info("未知客户端，连接code不正确...")
					return
				}
				// 存储c端的连接
				s.clientJoin(packet.Name, remoteAddr.IP.String(), remoteAddr)
				// 下发签名
				s.replyConnect(remoteAddr)

			case CommandPut:
				Info("接收发送来的数据...")
				if !SignCheck(remoteAddr.String(), packet.Sign) {
					s.ReplyPut(remoteAddr, 0, 1)
				} else {
					putData := &PutData{}
					bErr := ByteToObj(packet.Data, &putData)
					if bErr != nil {
						Error("解析put err :", bErr)
					}
					Info("putData.Label -> ", putData.Label)
					if fn, ok := s.PutHandle[putData.Label]; ok {
						fn(s, putData.Body)
					}
					s.ReplyPut(remoteAddr, putData.Id, 0)
				}

			case CommandGet:
				Info("接收Get请求...")
				if !SignCheck(remoteAddr.String(), packet.Sign) {
					s.ReplyPut(remoteAddr, 0, 1)
				} else {
					getData := &GetData{}
					boErr := ByteToObj(packet.Data, &getData)
					if boErr != nil {
						Error("解析put err :", boErr)
					}
					if fn, ok := s.GetHandle[getData.Label]; ok {
						code, rse := fn(s, getData.Param)
						getData.Response = rse
						gb, gbErr := ObjToByte(getData)
						if gbErr != nil {
							Error("对象转字节错误...")
						}
						s.ReplyGet(remoteAddr, getData.Id, code, gb)
					}
				}

			case CommandNotice: // 收到客户端通知的响应
				Info("收到客户端通知的响应...")
				if !SignCheck(remoteAddr.String(), packet.Sign) {
					s.ReplyPut(remoteAddr, 0, 1)
				} else {
					notice := &NoticeData{}
					bErr := ByteToObj(packet.Data, &notice)
					if bErr != nil {
						Error("返回的包解析失败， err = ", bErr)
					}
					if v, ok := NoticeDataMap.Load(notice.Id); ok {
						v.(*NoticeData).ctxChan <- true
					}
				}

			case CommandReply: // 收到客户端的回复
				Info("client端的回复...")
				if !SignCheck(remoteAddr.String(), packet.Sign) {
					s.ReplyPut(remoteAddr, 0, 1)
					break
				}
				reply := &Reply{}
				bErr := ByteToObj(packet.Data, &reply)
				if bErr != nil {
					Error("返回的包解析失败， err = ", bErr)
				}
				// Info("收到包 id: ", reply.Type)
				switch CommandCode(reply.Type) {
				case CommandGet:
					Info("请求 ID: %d | StateCode: %d", reply.CtxId, reply.StateCode)
					getData := &GetData{}
					boErr := ByteToObj(reply.Data, &getData)
					if boErr != nil {
						Error("解析put err :", boErr)
					}
					Info("getData.Label -> ", getData.Label)
					getF, _ := GetDataMap.Load(getData.Id)
					getF.(*GetData).Response = getData.Response
					getF.(*GetData).ctxChan <- true
				}

			default:
				// 未知包丢弃
				Info("未知包!!!")
				return
			}
		}()
	}
}

func (s *Servers) Write(client *net.UDPAddr, data []byte) {
	_, err := s.Conn.WriteToUDP(data, client)
	if err != nil {
		Error(err.Error())
	}
}

func (s *Servers) Get(funcLabel, name string, param []byte) ([]byte, error) {
	return s.GetAtNameTimeOut(DefaultSGetTimeOut, funcLabel, name, param)
}

func (s *Servers) GetAtNameTimeOut(timeOut int, funcLabel, name string, param []byte) ([]byte, error) {
	ip, ok := s.GetClientConn(name)
	if !ok && len(ip) < 1 {
		return nil, fmt.Errorf("客户端连接不存在")
	}
	return s.get(timeOut, funcLabel, name, ip[0].IP, param)
}

func (s *Servers) GetAtIP(funcLabel, name, ip string, param []byte) ([]byte, error) {
	return s.get(DefaultSGetTimeOut, funcLabel, name, ip, param)
}

func (s *Servers) GetAtIPTimeOut(timeOut int, funcLabel, name, ip string, param []byte) ([]byte, error) {
	return s.get(timeOut, funcLabel, name, ip, param)
}

// Get  向指定 client获取数据，  针对name,ip, 获取指定name或ip Client的数据
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
		Error("ObjToByte err = ", err)
	}

	c, ok := s.GetClientConnFromIP(name, ip)
	if !ok {
		return nil, fmt.Errorf("客户端连接不存在")
	}
	sign := SignGet(c.String())
	packet, err := PacketEncoder(CommandGet, s.name, sign, s.secretKey, b)
	if err != nil {
		Error(err)
	}
	s.Write(c, packet)
	select {
	case <-getData.ctxChan:
		Info("收到get返回的数据...")
		res := getData.Response
		GetDataMap.Delete(getData.Id)
		return res, nil
	case <-time.After(time.Millisecond * time.Duration(timeOut)):
		GetDataMap.Delete(getData.Id)
		return nil, ErrSGetTimeOut(funcLabel, name, ip)
	}
}

type NoticeRetry struct {
	TimeOutTimer time.Duration // 通知消息超时时间 10s > 重试*重试时间
	MaxRetry     int           // 通知消息最大重试次数
	RetryTimer   time.Duration // 重试等待时间
}

// SetNoticeRetry retryTimer 单位ms
func (s *Servers) SetNoticeRetry(maxRetry, retryTimer int) *NoticeRetry {
	return &NoticeRetry{
		MaxRetry:     maxRetry,
		RetryTimer:   time.Millisecond * time.Duration(retryTimer),
		TimeOutTimer: time.Millisecond * time.Duration((maxRetry+1)*retryTimer),
	}
}

// Notice  通知方法:针对 name,对Client发送通知
// 特点: 1. 重试次数 2. 指定时间内重试
func (s *Servers) Notice(name, label string, data []byte, retryConf *NoticeRetry) (string, error) {
	Info("发送通知消息......")
	if name == "" {
		name = formatName(DefaultClientName)
	}
	if retryConf == nil {
		retryConf = s.SetNoticeRetry(DefaultNoticeMaxRetry, DefaultNoticeRetryTimer)
	}
	// 直接下发消息，等待c端应答
	client, ok := s.GetClientConn(name)
	Info("client = ", name, client, ok)
	if !ok {
		return "未找到客户端", ErrNotFondClient(name)
	}
	// 组建通知包
	packetMap := make(map[*net.UDPAddr]*NoticeData)
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
				timer := time.NewTimer(retryConf.TimeOutTimer)
				select {
				case <-noticeData.ctxChan:
					Info("收到client通知反馈... id = ", noticeData.Id)
					NoticeDataMap.Delete(noticeData.Id)
					return
				case <-timer.C: // 超过10秒，表示已经大于最大重试的时间，释放内存
					NoticeDataMap.Delete(noticeData.Id)
					Info("超过最大重试的时间，释放内存")
					return
				}
			}

		}()
	}
	if s.noticeSend(packetMap) {
		return "通知下发完成", nil
	}
	retry := 1 // 重试次数
	for {
		if retry > retryConf.MaxRetry {
			// TODO 找到是哪个节点未收到通知
			return "重试次数完，还有客户端未收到通知", fmt.Errorf("重试次数完，还有客户端未收到通知")
		}
		timer := time.NewTimer(retryConf.RetryTimer) // 3秒重试一次
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
		Info("发送消息id = ", v.Id, "  -> has = ", has)
		if has {
			finish = false
			b, err := ObjToByte(v)
			if err != nil {
				Error("ObjToByte err = ", err)
			}
			sign := SignGet(cConn.String())
			packet, err := PacketEncoder(CommandNotice, s.name, sign, s.secretKey, b)
			if err != nil {
				Error(err)
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

func (s *Servers) replyConnect(client *net.UDPAddr) {
	sign := createSign()
	Info("生成签名 : ", sign)
	reply := &Reply{
		Type:      int(CommandConnect),
		Data:      []byte(sign),
		CtxId:     0,
		StateCode: 0,
	}
	b, e := ObjToByte(reply)
	if e != nil {
		Error(" e= ", e)
	}
	data, err := PacketEncoder(CommandReply, s.name, sign, s.secretKey, b)
	if err != nil {
		Error(err)
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
		Error("打包数据失败, e= ", e)
	}
	sign := SignGet(client.String())
	data, err := PacketEncoder(CommandReply, s.name, sign, s.secretKey, b)
	if err != nil {
		Error(err)
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
		Error("打包数据失败, e= ", e)
	}
	sign := SignGet(client.String())
	data, err := PacketEncoder(CommandReply, s.name, sign, s.secretKey, b)
	if err != nil {
		Error(err)
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

func (s *Servers) PutHandleFunc(label string, f func(s *Servers, body []byte)) {
	if _, ok := s.PutHandle[label]; ok {
		PanicPutHandleFuncExist(label)
	}
	s.PutHandle[label] = f
}

func (s *Servers) GetHandleFunc(label string, f func(s *Servers, param []byte) (int, []byte)) {
	if _, ok := s.GetHandle[label]; ok {
		PanicGetHandleFuncExist(label)
	}
	s.GetHandle[label] = f
}

func (s *Servers) GetServersName() string {
	return s.name
}

func (s *Servers) clientJoin(name, ip string, addr *net.UDPAddr) {
	Info("将客户端加入clientJoin... ", name, len(name))
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
		Info("客户端存在...")
		return
	}
	s.CMap[name] = append(s.CMap[name], client)
	s.onLineTable[fmt.Sprintf("%s@%s", name, ip)] = true
	return
}

func (s *Servers) ClientDiscard(name, ip string) {
	Info("删除离线的c")
	if name == "" {
		name = formatName(DefaultClientName)
	}
	if v, ok := s.CMap[name]; ok {
		for i, c := range v {
			Info(i, c.IP, ip)
			if c.IP == ip {
				v = append(v[:i], v[i+1:]...)
			}
		}
		s.CMap[name] = v
		s.onLineTable[fmt.Sprintf("%s|%s", name, ip)] = false
	}
}

func (s *Servers) GetClientConn(name string) ([]*ClientConnectObj, bool) {
	if name == "" {
		name = DefaultClientName
	}
	name = formatName(name)
	Info("获取 client ... ", name, len(name))
	if v, ok := s.CMap[name]; ok && len(v) > 0 {
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
		tTime := time.Duration(ServersTimeWheel)
		for {
			timer := time.NewTimer(tTime * time.Second)
			select {
			case <-timer.C:
				//Info("时间轮检查c端的连接...")
				t := time.Now().Unix()
				for k, v := range s.CMap {
					for _, c := range v {
						if t-c.Last > HeartbeatTimeLast { // 这个时间要大于5秒，因为来自c端的心跳就是5秒
							InfoF("离线服务器名称:%s IP地址:%s  当前t=%d last=%d", k, c.IP, t, c.Last)
							s.ClientDiscard(k, c.IP)
						} else {
							//InfoF("在线服务器名称:%s IP地址:%s  当前t=%d last=%d", k, c.IP, t, c.Last)
						}
					}
				}
			}
		}
	}()
}

// OnLineTable 获取当前客户端连接情况
func (s *Servers) OnLineTable() map[string]bool {
	for k, v := range s.onLineTable {
		kList := strings.Split(k, "@")
		onLine := "在线"
		if !v {
			onLine = "离线"
		}
		info := fmt.Sprintf("name:%s | ip:%s --> %s", kList[0], kList[1], onLine)
		Info(info)
	}
	return s.onLineTable
}

// TODO ... 拒绝指定客户端的通讯

type ClientConnectObj struct {
	IP   string
	Addr *net.UDPAddr
	Last int64 // 最后一次连接的时间
}
