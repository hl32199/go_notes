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