package udp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
)

// 包设计 总包大小 548字节
// 第1字节  1字节 指令确认是什么数据类型
// 第2~8字节 7字节  名字  --- 这个设计是确认谁是谁 名字对应多个ip 每个ip对应一个conn  name[{ip1:conn},{ip2:conn}]
// 9~16字节  7字节  签名由对方签发，相互保存，随时会变
// 第16~548 533字节  字节是数据内容
// 数据使用 des加密
// 使用gzip压缩

// PacketEncoder 封包
func PacketEncoder(cmd CommandCode, name, sign string, data []byte) ([]byte, error) {
	var (
		err    error
		stream []byte
		buf    = new(bytes.Buffer)
	)
	_ = binary.Write(buf, binary.LittleEndian, cmd)
	ln := len(name)
	if ln > 0 && ln <= 7 {
		// 补齐位
		for i := 0; i < 7-ln; i++ {
			name += " "
		}
		_ = binary.Write(buf, binary.LittleEndian, []byte(name))
	} else if ln > 7 {
		return nil, fmt.Errorf("name length above 7")
	} else {
		_ = binary.Write(buf, binary.LittleEndian, []byte("0000000"))
	}
	if len(sign) != 7 {
		_ = binary.Write(buf, binary.LittleEndian, []byte("0000000"))
	} else {
		_ = binary.Write(buf, binary.LittleEndian, []byte(sign))
	}
	if len(data) > 450 {
		return nil, fmt.Errorf("数据大于 540个字节, 建议拆分.")
	}
	err = binary.Write(buf, binary.LittleEndian, data)
	if err != nil {
		return stream, err
	}
	stream = buf.Bytes()
	return stream, nil
}

// PacketDecrypt 解包
func PacketDecrypt(data []byte, n int) {
	if n < 15 {
		fmt.Println("空包")
		return
	}
	command := CommandCode(uint8(data[0:1][0]))
	fmt.Println("command = ", command)
	name := data[1:8]
	fmt.Println("name = ", string(name))
	sign := string(data[8:15])
	fmt.Println("sign = ", string(sign))
	txt := data[15:n]
	fmt.Println("txt = ", string(txt))

	switch command {
	case CommandConnect:
		log.Println("请求连接...")
		// TODO 下发签名

	case CommandPut:
		log.Println("接收发送来的数据...")
		// TODO 验证签名
		if sign != "1234567" {
			log.Println("未知客户端")
		}

	case CommandHeartbeat:
		log.Println("接收到心跳包...")

	}

}

// des加密解密
