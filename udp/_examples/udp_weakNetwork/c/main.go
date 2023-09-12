package main

import (
	"fmt"
	"github.com/mangenotwork/beacon-tower/udp"
	"time"
)

func main() {
	// 定义客户端
	client, err := udp.NewClient("192.168.3.86:12347")
	if err != nil {
		panic(err)
	}
	// 每两秒发送一些测试数据
	go func() {
		n := 0
		for {
			n++
			time.Sleep(2 * time.Second)
			// put上传数据到服务端的 case2 方法
			client.Put("case", []byte(fmt.Sprintf("%d | hello : %d", time.Now().UnixNano(), n)))
			udp.Info("n = ", n)
		}
	}()

	// 运行客户端
	client.Run()
}
