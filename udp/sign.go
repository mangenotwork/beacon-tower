package udp

import "sync"

// 存储下发的签名，异步持久化到磁盘
var signMap sync.Map

func SignStore(addr, sign string) {
	signMap.Store(addr, sign)
}

func SignCheck(addr, sign string) bool {
	v, _ := signMap.Load(addr)
	if v.(string) == sign {
		return true
	}
	return false
}

func SignGet(addr string) string {
	v, _ := signMap.Load(addr)
	return v.(string)
}

// Persistent TODO 持久存储
func Persistent() {
}
