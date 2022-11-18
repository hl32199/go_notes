## 变量名
字母或下划线开头,后面可跟任意数量字符、数字、下划线，区分大小写。  
以下命名是可以的：
```go
package main
var _123abc,_3,我,_你 string = "aa","bb","cc","dd"
``` 

内置函数名可以用作名称，但是某些情况会出错，比如：
```go
    a := make(map[string]struct{})
	len := "aaa"
	make := "bbb"
	a[len] = struct{}{}
	a[make] = struct{}{}
	fmt.Println(a)
//输出 map[aaa:{} bbb:{}]
```
但是把变量命名为"make",会导致make内置函数无法调用：
```go
	make := "bbb"
	a := make(map[string]struct{})
	a[make] = struct{}{}
	fmt.Println(a)
//输出
//.\test.go:7:11: cannot call non-function make (type string), declared at .\test.go:6:7
//.\test.go:7:12: type map[string]struct {} is not an expression
```
为避免不必要的麻烦，应使用常规的命名，以下划线或字母开头，只包含字母数字和下划线。

## new函数
内置函数，返回指定类型的指针，值初始化为指定类型的零值。
直接声明的指定类型的指针类型，指针初始化为nil。
```go
p := new(int)
```
等价于
```go
var i int
p := &i
```

## 短变量声明陷阱
短变量声明（:=）左侧的变量，至少有一个是在当前作用域内未声明的。在当前作用域内已声明的变量会被赋值。
当前作用域内未声明的变量，会被声明并赋值，即使在外层作用域有同名变量。此时当前和外部作用域的同名变量是互相独立的。
例：
```go
	var num int = 1
	var err error
	fmt.Println(num,err)

	if num,err := 2,errors.New("test");err != nil {
		fmt.Println(num,err)
	}
	fmt.Println(num,err)

	if num,err = 3,errors.New("test2");err != nil {
		fmt.Println(num,err)
	}
	fmt.Println(num,err)
```
输出
```
1 <nil>
2 test
1 <nil>
3 test2
3 test2
```

## 算术运算
```go
	fmt.Println(-5%3) //-2
	fmt.Println(-5%-3) //-2
	fmt.Println(5/4) //1
	fmt.Println(5.0/4.0) //1.25
```