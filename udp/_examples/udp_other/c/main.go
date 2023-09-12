package main

import (
	"fmt"
	"github.com/mangenotwork/beacon-tower/udp"
)

func main() {
	// 定义客户端
	client, err := udp.NewClient("192.168.3.86:12345")
	if err != nil {
		panic(err)
	}
	// get方法
	client.GetHandleFunc("getClient", CGetTest)
	// 通知方法
	client.NoticeHandleFunc("testNotice", CNoticeTest)

	//go func() {
	//	n := 0
	//	for {
	//		n++
	//		// put上传数据到服务端的 case2 方法
	//		client.Put("putCase", []byte(fmt.Sprintf("%d | hello : %d", time.Now().UnixNano(), n)))
	//		udp.Info("n = ", n)
	//	}
	//}()

	// 运行客户端
	client.Run()
}

func CGetTest(c *udp.Client, param []byte) (int, []byte) {
	udp.Info("获取到的请求参数  param = ", string(param))
	return 0, []byte(fmt.Sprintf("客户端名称 %s.", c.DefaultClientName))
}

func CNoticeTest(c *udp.Client, data []byte) {
	udp.Info("收到来自服务器的通知，开始执行......")
	udp.Info("data = ", string(data))
}
