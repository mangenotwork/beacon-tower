package udp

import (
	"bufio"
	"fmt"
	"github.com/mangenotwork/common/log"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"
)

// backlog 积压的数据，所有发送的数据都会到这里，只有服务端确认的数据才会被删除
var backlog sync.Map
var backlogCount int64 = 0
var backlogCountMax int64 = 10
var backlogCountMin int64 = 5
var backlogFile = "%d.udb"

func backlogAdd(putId int64, putData PutData) {
	atomic.AddInt64(&backlogCount, 1)
	backlog.Store(putId, putData)
	backlogStorage()
}

func backlogDel(putId int64) {
	atomic.AddInt64(&backlogCount, -1)
	backlog.Delete(putId)
}

// backlogStorage 持久化方案: 保护内存不持续增长,尽力保证server掉线后数据不丢失，监听非强制kill把数据持久化
// 只有当积压数据条数大于设定值(backlogCount > max)就将当前所有积压的数据持久化到磁盘，释放内存存放新的数据
// 当积压数据条数小于设定值(backlogCount < min)就把持久化数据写到积压内存
// 当监听到非强制kill把数据持久化
func backlogStorage() {
	if backlogCount > backlogCountMax {
		log.Error("触发持久化...... backlogCount = ", backlogCount)
		toUdb()
	}
}

func toUdb() {
	if backlogCount < 1 {
		return
	}
	file, err := os.OpenFile(fmt.Sprintf(backlogFile, time.Now().Unix()), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Error(err)
	}
	defer func() {
		_ = file.Close()
	}()
	backlog.Range(func(key, value any) bool {
		vb, err := ObjToByte(value)
		_, err = file.Write(vb)
		_, err = file.Write([]byte("\n"))
		if err != nil {
			log.Error(err)
		}
		backlogDel(key.(int64))
		return true
	})
}

func TBacklog() {
	go func() {
		for {
			time.Sleep(1 * time.Second)
			backlog.Range(func(key, value any) bool {
				log.Info("消费..... backlogCount : ", backlogCount)
				backlogDel(key.(int64))
				return true
			})
		}
	}()
}

// BacklogLoad 加载持久化数据 并消费
func BacklogLoad() {
	log.Error("加载持久化数据 并消费 backlogCount = ", backlogCount)
	if backlogCount > backlogCountMin {
		log.Error("当前 队列 大于触发条件不加载 : ", backlogCount)
		return
	}
	files, err := ioutil.ReadDir(".")
	if err != nil {
		log.Error("error reading directory:", err)
		return
	}
	for _, file := range files {
		extension := path.Ext(file.Name())
		if extension == ".udb" {
			filePath := "./" + file.Name()
			// 删掉没用的文件
			if file.Size() == 0 {
				err := os.Remove(filePath)
				if err != nil {
					log.Error(err)
					return
				}
			}
			if file.Size() > 0 {
				log.Info(file.Name())
				fileToBacklog(filePath)
				if backlogCount > backlogCountMin {
					break
				}
			}
		}
	}
}

func fileToBacklog(fName string) {
	f, err := os.Open(fName)
	if err != nil {
		log.Error(err)
		return
	}
	putDataList1 := make([]PutData, 0)
	putDataList2 := make([]PutData, 0)
	var n int64 = 0
	reader := bufio.NewReader(f)
	for {
		n++
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		//log.Info(string(line))
		putData := PutData{}
		err = ByteToObj(line, &putData)
		if err != nil {
			log.Error(err)
			continue
		}
		//log.Info("持久化 putData = ", putData)
		if n < int64(backlogCountMax/2)+1 {
			putDataList1 = append(putDataList1, putData)
		} else {
			putDataList2 = append(putDataList2, putData)
		}
		if err != nil {
			log.Error(err)
			return
		}
	}
	for _, v := range putDataList1 {
		log.Error("加入 数据到队列 ... ")
		backlogAdd(v.Id, v)
		log.Error("加入后的count = ", backlogCount)
	}
	_ = f.Close()
	resetBacklogFile(fName, putDataList2)
}

func resetBacklogFile(fName string, putDataList []PutData) {
	err := os.Remove(fName)
	if err != nil {
		log.Error(err)
		return
	}
	if len(putDataList) < 1 {
		return
	}
	file, err := os.OpenFile(fName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Error(err)
		return
	}
	defer file.Close()
	for _, v := range putDataList {
		vb, err := ObjToByte(v)
		_, err = file.Write(vb)
		_, err = file.Write([]byte("\n"))
		if err != nil {
			log.Error(err)
		}
	}
}
