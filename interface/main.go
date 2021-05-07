package main

import (
	"log"
)

type MyErr struct {
	Msg string
}

func main() {
	var err, err1 error
	var myErr *MyErr

	err = getErr()
	log.Printf("%#v", err)
	log.Println(err == nil)

	err1 = GetMyErr()
	log.Printf("%#v", err1)
	log.Println(err1 == nil)

	myErr = GetMyErr()
	log.Printf("%#v", myErr)
	log.Println(myErr == nil)

	log.Println(myErr.Error())
	log.Println(err1.Error())
	log.Println(err.Error())
}

func getErr() error {
	return nil
}
func GetMyErr() *MyErr {
	return nil
}

func (m *MyErr) Error() string {
	return "===test nil interface==="
}

/**
----------------解析-------------------
输出
2021/05/07 14:57:41 <nil>
2021/05/07 14:57:41 true
2021/05/07 14:57:41 (*main.MyErr)(nil)
2021/05/07 14:57:41 false
2021/05/07 14:57:41 (*main.MyErr)(nil)
2021/05/07 14:57:41 true
2021/05/07 14:57:41 ===test nil interface===
2021/05/07 14:57:41 ===test nil interface===
panic: runtime error: invalid memory address or nil pointer dereference
[signal 0xc0000005 code=0x0 addr=0x0 pc=0xab2ceb]

goroutine 1 [running]:
main.main()
        D:/go/go_demo/demo6/main.go:29 +0x2ab
exit status 2

声明但未赋值或直接赋值为nil的 interface，和 赋值为一个具体的实现类型（但实现类型为nil）的情况是不一样的。
一个interface{}类型的变量包含了2个指针，一个指针指向值的在编译时确定的类型，另外一个指针指向实际的值。
type InterfaceStructure struct {
  pt uintptr // 到值类型的指针
  pv uintptr // 到值内容的指针
}

asInterfaceStructure 将一个interface{}转换为InterfaceStructure
func asInterfaceStructure(i interface{}) InterfaceStructure {
  return *(*InterfaceStructure)(unsafe.Pointer(&i))

}

*/
