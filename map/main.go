package main

import "fmt"

func main() {
	//DeclareAndInit()
	//CheckKeyExist()
	DelMapKey()
}

//声明但未初始化的map，为nil
//map的key可以是定义了相等运算符的任何类型，例如整数，浮点数和复数，字符串，指针，接口（只要动态类型支持相等），结构和数组。
//对未初始化的map不能使用key来访问
func DeclareAndInit() {
	m1 := make(map[string]string)
	fmt.Printf("%#v\n", m1) // print:map[string]string{}
	m1["aa"] = "aaa"
	fmt.Printf("%#v\n", m1) // print:map[string]string{"aa":"aaa"}

	//访问不存在的key将获得值类型的零值
	v := m1["bb"]
	fmt.Printf("not exist key:%s\n", v) //print:not exist key:

	//对声明但未初始化的map,可以通过key取值,将得到值类型的零值，但是不能对key赋值
	var m2 map[string]string
	fmt.Printf("%#v\n", m2) // print:map[string]string(nil)
	v2 := m2["aa"]
	fmt.Printf("get value from uninitialed map:%s\n", v2) //print: get value from uninitialed map:
	m2["bb"] = "bbb"                                      // print:panic: assignment to entry in nil map
}

func CheckKeyExist() {
	m := make(map[string]string)
	m["aa"] = "aaa"
	//key存在时，ok为true;key不存在时，v1为值类型的零值，ok为false
	//这种方式可安全的用于为nil的map
	v1, ok := m["aa"]
	fmt.Println(v1, ok) // print:aaa true
	v2, ok := m["bb"]
	fmt.Println(v2, ok) // print: false

	var m2 map[string]string
	v3, ok := m2["aa"]
	fmt.Println(v3, ok) // print: false
}

// slice、map、function不能用作映射键,因为未定义比较操作符未定义
func InvalidKey() {
	//m := map[[]int]string
	//m2 := map[map[string]int]string

	//type functinTyoe func(int) bool
	//m3 := map[functinTyoe]string
}

//删除map的一个key,即使key不存在或map为nil，也是安全的
func DelMapKey() {
	m := map[string]string{"a": "aa", "b": "bb"}
	delete(m, "a")
	fmt.Printf("%#v\n", m)

	var m1 map[string]string
	delete(m1, "a")
	fmt.Printf("%#v\n", m1)
}
