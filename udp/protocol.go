package udp

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"crypto/des"
	"encoding/binary"
	"encoding/json"
	"github.com/mangenotwork/common/log"
	"io"
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
func PacketEncoder(cmd CommandCode, name, sign, secret string, data []byte) ([]byte, error) {
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
		return nil, ErrNmeLengthAbove
	} else {
		_ = binary.Write(buf, binary.LittleEndian, []byte("0000000"))
	}
	if len(sign) != 7 {
		_ = binary.Write(buf, binary.LittleEndian, []byte("0000000"))
	} else {
		_ = binary.Write(buf, binary.LittleEndian, []byte(sign))
	}
	log.Info("源数据 : ", len(data))
	// 压缩数据
	//d := GzipCompress(data)
	d := ZlibCompress(data)
	log.Info("压缩后数据长度: ", len(d))

	// 加密数据
	d = desECBEncrypt(d, []byte(secret))
	log.Info("加密数据 : ", len(d))

	if len(d) > 540 {
		log.Error(ErrDataLengthAbove)
	}
	err = binary.Write(buf, binary.LittleEndian, d)
	if err != nil {
		return stream, err
	}
	stream = buf.Bytes()
	return stream, nil
}

// PacketDecrypt 解包
func PacketDecrypt(secret string, data []byte, n int) (*Packet, error) {
	var err error
	if n < 15 {
		log.Info("空包")
		return nil, ErrNonePacket
	}
	command := CommandCode(data[0:1][0])
	name := string(data[1:8])
	sign := string(data[8:15])
	b := data[15:n]
	// 解密数据
	b = desECBDecrypt(b, []byte(secret))
	// 解压数据
	//b, err := GzipDecompress(data[15:n])
	b, err = ZlibDecompress(b)
	if err != nil {
		log.Error("解压数据失败 err: ", err)
		return nil, err
	}
	return &Packet{
		Command: command,
		Name:    name,
		Sign:    sign,
		Data:    b,
	}, nil
}

func ObjToByte(obj interface{}) ([]byte, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return []byte(""), err
	}
	return b, nil
}

func ByteToObj(data []byte, obj interface{}) error {
	return json.Unmarshal(data, obj)
}

// GzipCompress gzip压缩
func GzipCompress(src []byte) []byte {
	var in bytes.Buffer
	w, err := gzip.NewWriterLevel(&in, gzip.BestCompression)
	_, err = w.Write(src)
	err = w.Close()
	if err != nil {
		log.Error(err)
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
	return buf.Bytes(), err
}

// ZlibCompress zlib压缩
func ZlibCompress(src []byte) []byte {
	buf := new(bytes.Buffer)
	//根据创建的buffer生成 zlib writer
	writer := zlib.NewWriter(buf)
	//写入数据
	_, err := writer.Write(src)
	err = writer.Close()
	if err != nil {
		log.Error(err)
	}
	return buf.Bytes()
}

// ZlibDecompress zlib解压
func ZlibDecompress(src []byte) ([]byte, error) {
	reader := bytes.NewReader(src)
	gr, err := zlib.NewReader(reader)
	if err != nil {
		return []byte(""), err
	}
	bf := make([]byte, 0)
	buf := bytes.NewBuffer(bf)
	_, err = io.Copy(buf, gr)
	err = gr.Close()
	return buf.Bytes(), err
}

// des ecb 加密解密
// 占定使用这种加密方法，保障抓包数据不是明文

func pkcs5Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	text := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, text...)
}

func pkcs5UnPadding(origData []byte) []byte {
	length := len(origData)
	unPadding := int(origData[length-1])
	return origData[:(length - unPadding)]
}

func desECBEncrypt(data, key []byte) []byte {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil
	}
	bs := block.BlockSize()
	data = pkcs5Padding(data, bs)
	if len(data)%bs != 0 {
		return nil
	}
	out := make([]byte, len(data))
	dst := out
	for len(data) > 0 {
		block.Encrypt(dst, data[:bs])
		data = data[bs:]
		dst = dst[bs:]
	}
	return out
}

func desECBDecrypt(data, key []byte) []byte {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil
	}
	bs := block.BlockSize()
	if len(data)%bs != 0 {
		return nil
	}
	out := make([]byte, len(data))
	dst := out
	for len(data) > 0 {
		block.Decrypt(dst, data[:bs])
		data = data[bs:]
		dst = dst[bs:]
	}
	out = pkcs5UnPadding(out)
	return out
}
