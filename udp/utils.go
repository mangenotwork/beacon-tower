package udp

import (
	"bytes"
	"encoding/binary"
	"github.com/mangenotwork/common/utils"
)

func int64ToBytes(n int64) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	err := binary.Write(buf, binary.BigEndian, n)
	return buf.Bytes(), err
}

func intToBytes(n int) ([]byte, error) {
	return int64ToBytes(int64(n))
}

func bytesToInt(bys []byte) (int, error) {
	i, err := bytesToInt64(bys)
	return int(i), err
}

func bytesToInt64(bys []byte) (int64, error) {
	buf := bytes.NewBuffer(bys)
	var data int64
	err := binary.Read(buf, binary.BigEndian, &data)
	return data, err
}

func id() int64 {
	return utils.ID()
}

func formatName(str string) string {
	ln := len(str)
	if ln > 0 && ln <= 7 {
		// 补齐位
		for i := 0; i < 7-ln; i++ {
			str += " "
		}
	}
	return str
}
