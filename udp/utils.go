package udp

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_+=~!@#$%^&*()"

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func intToBytes(n int) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	err := binary.Write(buf, binary.BigEndian, int64(n))
	return buf.Bytes(), err
}

func bytesToInt(bys []byte) (int, error) {
	buf := bytes.NewBuffer(bys)
	var data int64
	err := binary.Read(buf, binary.BigEndian, &data)
	return int(data), err
}
