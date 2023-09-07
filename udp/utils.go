package udp

import (
	"bytes"
	"encoding/binary"
)

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
