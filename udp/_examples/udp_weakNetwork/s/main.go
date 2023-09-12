package main

import (
	"github.com/mangenotwork/beacon-tower/udp"
	"os"
)

/*

测试积压步骤
1. 启动s
2. 启动c
3. 关闭s 等一会
4. 启动s
5. 检测 test.txt 接收到的数据


*/

var testFile = "test.txt"

func main() {
	s, err := udp.NewServers("0.0.0.0", 12347)
	if err != nil {
		panic(err)
	}
	s.PutHandleFunc("case", Case)

	// 启动servers
	s.Run()
}

func Case(s *udp.Servers, body []byte) {
	udp.Info("Case2 func --> ", string(body))
	udp.Info("[Client 测试put] passed")
	file, err := os.OpenFile(testFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		udp.Error(err)
	}
	defer func() {
		_ = file.Close()
	}()
	content := []byte(string(body) + "\n")
	_, err = file.Write(content)
	if err != nil {
		panic(err)
	}
}
