package udp

type PutData struct {
	Label string // 标签，用于区分当前数据处理的方法
	Body  []byte // 传过来的数据
}

type ServersPutFunc map[string]func(s *Servers, data []byte)
