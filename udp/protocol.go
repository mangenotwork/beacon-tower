package udp

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
)

// Packet 包设计 总包大小 548字节
// 第1字节  1字节 指令确认是什么数据类型
// 第2~8字节 7字节  名字  --- 这个设计是确认谁是谁 名字对应多个ip 每个ip对应一个conn  name[{ip1:conn},{ip2:conn}]
// 9~16字节  7字节  签名由对方签发，相互保存，随时会变
// 第16~548 533字节  字节是数据内容
// 数据使用 des加密
// 使用gzip压缩
type Packet struct {
	Command CommandCode
	Name    string
	Sign    string
	Data    []byte
}

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
func PacketDecrypt(data []byte, n int) *Packet {
	if n < 15 {
		fmt.Println("空包")
		return nil
	}
	command := CommandCode(data[0:1][0])
	name := data[1:8]
	sign := string(data[8:15])
	return &Packet{
		Command: command,
		Name:    string(name),
		Sign:    string(sign),
		Data:    data[15:n],
	}
}

// DataEncoder 数据量大，使用 json 序列化+gzip压缩
func DataEncoder(obj interface{}) ([]byte, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return []byte(""), err
	}
	return GzipCompress(b), nil
}

// DataDecoder 解码
func DataDecoder(data []byte, obj interface{}) error {
	b, err := GzipDecompress(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, obj)
}

// GzipCompress gzip压缩
func GzipCompress(src []byte) []byte {
	var in bytes.Buffer
	w, err := gzip.NewWriterLevel(&in, gzip.BestCompression)
	_, err = w.Write(src)
	err = w.Close()
	if err != nil {
		log.Println(err)
	}
	return in.Bytes()
}

// GzipDecompress gzip解压
func GzipDecompress(src []byte) ([]byte, error) {
	reader := bytes.NewReader(src)
	gr, err := gzip.NewReader(reader)
	if err != nil {
		return []byte(""), err
	}
	bf := make([]byte, 0)
	buf := bytes.NewBuffer(bf)
	_, err = io.Copy(buf, gr)
	err = gr.Close()
	return buf.Bytes(), nil
}

// des加密解密
