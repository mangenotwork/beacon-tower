package main

import (
	"beacon-tower/udp"
	"fmt"
	"time"
)

func main() {
	// 定义客户端
	client := udp.NewClient("192.168.0.103:12345")

	// get方法
	client.GetHandleFunc("getClient", CGetTest)
	// 通知方法
	client.NoticeHandleFunc("testNotice", CNoticeTest)

	// 每两秒发送一些测试数据
	go func() {
		n := 0
		for {
			n++
			time.Sleep(2 * time.Second)

			// put上传数据到服务端的 case2 方法
			client.Put("case2", []byte(fmt.Sprintf("%d | hello : %d", time.Now().UnixNano(), n)))
			udp.Info("n = ", n)

			// get请求服务端的 case3 方法
			rse, err := client.Get("case3", []byte("test"))
			if err != nil {
				udp.Error(err)
				udp.Info("[Client 测试get] failed")
				continue
			}
			udp.Info("get 请求返回 = ", string(rse))
			udp.Info("[Client 测试get] passed")
		}
	}()

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

var txt = `hello; 你好这是一个测试数据,哈哈哈哈哈哈，你好你好世界,aaaaaaa!!!!!
你好这是一个测试数据,哈哈哈哈哈哈，你好你好世界,aaaaaaa!!!!!
你好这是一个测试测试数据,哈哈哈哈哈哈，你好你好世界,aaaaaaa!!!!!
你好这是一个测试数据,哈哈哈哈哈哈，你好你好世界,aaaaaaa!!!!!
你好这是一个测试数据,哈哈哈哈哈哈，你好你好世界,aaaaaaa!!!!!
你好这是一个测试数据,哈哈哈哈哈哈，你好你好世界,aaaaaaa!!!!!
asdasdiugpqbrpqwphuhi231241bjguj12lj0712b3bv2ug12v3g12vkjl
2132o1j0y8412b3lsf1s56f4ds5f13sd21rurh1rbsjadnsladAHAHJLANS-3R
3R4WE62F1S84DFSDF0SDUFDSBFBSDKB2Y3F1V11BKNLMSLFSD-FSDHFUSDFBJH
ASAD56A4FAS12F593223N;KM;DSF-SDF=1=__)(+_)}{|uyiygvgyrch
ASDASDASKLKERQWKLNLQWKNQWK4894123134841321353513213215351231
56sad465asd46f54as6f54a65sa4f65asf46as5f46asf4fa65f46sa5f46as5f46as5f4fsa65f
asfsf54a56sf4a6s5f4a65f465fs4f65fs789fs4sa6f1fs321fs65s46s5a1fsa2f156f1f651f56
asfasf65afs6fas2sf+a62fsa+62f2asf65as1f65asf1f64f56asf1a65f16a5s1f65asf1as65f1as6f5
asf56a1sf6as5f16a5sf1as65f1sa65f1as6f5a1sf65as1f6as1f2asf1as65f1
asf156asf1as2fa0s23f1as56f1as32fasf35as1fas5fas32f1as3f51as5fa1s35as1
as51f5as1f6as1fa65sf1as65f1asasdasdsadsadsad78791279831274982144156241654
1231245612465211s6a516as51fas651dsa神鼎飞丹砂弄欧广东省防范我去【我房间不不卡深V巅峰赛构成【怕吧奥sad`
