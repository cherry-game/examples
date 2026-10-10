package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	cfacade "github.com/cherry-game/cherry/facade"
	pomeloMessage "github.com/cherry-game/cherry/net/parser/pomelo/message"
)

type Student struct {
	Name    string
	Age     uint8
	Address string
}

func encode(v interface{}) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(v)
	return buffer.Bytes(), err
}

func decode(b []byte, v interface{}) error {
	decoder := gob.NewDecoder(bytes.NewReader(b)) //创建解密器
	return decoder.Decode(v)
}

func TestGOB(t *testing.T) {
	//序列化
	s1 := Student{
		"张三",
		18,
		"江苏省",
	}

	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer) //创建编码器
	err1 := encoder.Encode(&s1)        //编码
	if err1 != nil {
		log.Panic(err1)
	}

	fmt.Printf("序列化后：%x\n", buffer.Bytes())

	//反序列化
	byteEn := buffer.Bytes()
	decoder := gob.NewDecoder(bytes.NewReader(byteEn)) //创建解密器
	var s2 Student
	err2 := decoder.Decode(&s2) //解密
	if err2 != nil {
		log.Panic(err2)
	}
	fmt.Println("反序列化后：", s2)
}

func TestMessage(t *testing.T) {
	gob.Register(context.TODO())
	gob.Register(pomeloMessage.Message{})
	// cherry v1.5+ 起 cfacade.Message.Args 由 []interface{} 改为 interface{}
	// 以接口承载切片时, gob 需要显式注册切片的具体类型才能编解码
	gob.Register([]interface{}{})

	ctx := context.TODO()
	msg := pomeloMessage.Message{
		Type:  1,
		ID:    2,
		Route: "333",
		Data:  nil,
		Error: false,
	}

	m := cfacade.Message{
		FuncName: "test",
		Args: []interface{}{
			ctx,
			nil,
			msg,
		},
	}

	mBytes, err := encode(&m)
	fmt.Println(err)

	m1 := cfacade.Message{}
	err = decode(mBytes, &m1)
	fmt.Println(err)
}

type FileAll struct {
	Name string
	Cxt  []byte
}

// Test111 是一个手动运行的小工具: 把一个文件go序列化后输出到stdout。
// 它需要显式传入文件路径, 例如: go test -run Test111 ./test_gob -- myfile.txt
// `go test ./test_gob` 时 go 命令会把自己的标志位(如 -test.testlogfile)当作第一个参数传进来,
// 这里做一次校验, 没有传入真实文件路径时跳过, 避免误报失败。
func Test111(t *testing.T) {
	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], "-") {
		t.Skip("需要传入文件路径, 例如: go test -run Test111 ./test_gob -- myfile.txt")
	}

	var fa1 FileAll
	var err error
	fa1.Name = os.Args[1]
	fa1.Cxt, err = os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	enc := gob.NewEncoder(os.Stdout)
	enc.Encode(fa1)
}
