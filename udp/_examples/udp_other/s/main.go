package main

import (
	"beacon-tower/udp"
)

func main() {
	// 初始化 s端
	s, err := udp.NewServers("0.0.0.0", 12345)
	if err != nil {
		panic(err)
	}
	s.PutHandleFunc("putCase", PutCase)

	//go func() {
	//
	//	for {
	//		// 发送一个通知 [测试put]  BUG:延迟严重
	//		//rse, rseErr := s.Notice("", "testNotice", []byte("testNotice"), nil)
	//		//if rseErr != nil {
	//		//	udp.Error(rseErr)
	//		//	udp.Info("[Servers 测试notice] failed")
	//		//	continue
	//		//}
	//		//udp.Info("[Servers 测试notice] passed")
	//		//udp.Info(rse)
	//
	//		// 异步发送
	//		go func() {
	//			rse, rseErr := s.Notice("", "testNotice", []byte("testNotice"),
	//				s.SetNoticeRetry(2, 10))
	//			if rseErr != nil {
	//				udp.Error(rseErr)
	//				udp.Info("[Servers 测试notice] failed")
	//			}
	//			udp.Info("[Servers 测试notice] passed")
	//			udp.Info(rse)
	//		}()
	//
	//		// 发送get
	//		//getRse, err := s.Get("getClient", "", []byte("getClient"))
	//		//if err != nil {
	//		//	udp.Info("[Servers 测试get] failed")
	//		//	continue
	//		//}
	//		//udp.Info(string(getRse), err)
	//		//udp.Info("[Servers 测试get] passed")
	//	}
	//}()

	// 启动servers
	s.Run()
}

func PutCase(s *udp.Servers, body []byte) {
	udp.Info("PutCase func --> ", string(body))
	udp.Info("[Client 测试put] passed")
}
