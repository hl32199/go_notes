package main

import (
	"fmt"
)

func main() {
	//new(T) 为一个零值的T类型分配内存，并返回其指针，T类型的实例已经初始化为零值，可直接使用
	NewStruct()

	//slice map channel为内置的三种引用类型，这三种类型的零值为nil,只声明未初始化时，等于nil,需要初始化后才可以使用
	//一个特殊情况是 为nil的slice 可以作为 append(),len(),cap() 的参数，len() cap()返回0
	NewSlice()

	// make 只能用于slice map channel，返回已经初始化的对应类型实例
	var b = make([]int, 5) // make返回已初始化的切片，切片底层的数组已经初始化为零值
	fmt.Println(b)         // print:[0 0 0 0 0]

	// Unnecessarily complex:
	var c = new([]int)
	*c = make([]int, 5, 5)
	fmt.Println(c) // print:&[0 0 0 0 0]
	// Idiomatic（惯用语）:
	d := make([]int, 4)
	fmt.Println(d) // print:[0 0 0 0]

}

func NewStruct() {
	p := new(User) // type *User
	var v User     // type  User 声明以后已经被初始化为零值
	fmt.Println(p) //print:&{ 0}
	fmt.Println(v) //print:{ 0}
}

func NewSlice() {
	var a = new([]int) // 声明但未初始化的slice为nil,但可以获取长度和容量，长度容量都为0,这里返回零值的slice的指针，即 *p == nil;
	var aa []int
	aaa := &aa                              // aaa 等同于 a
	fmt.Println(a, len(*a), cap(*a))        // print:&[] 0 0
	fmt.Println(aa, len(aa), cap(aa))       // print:[] 0 0
	fmt.Println(aaa, len(*aaa), cap(*aaa))  // print:&[] 0 0
	fmt.Printf("%#v %#v %#v\n", a, aa, aaa) // print: &[]int(nil) []int(nil) &[]int(nil)
	aa = append(aa, 1)                      // 可以对未初始化的slice 执行 append 操作
	fmt.Printf("%#v\n", aa)                 // print: [] 0 0[]int{1}
}

type User struct {
	Name string
	Age  int
}
