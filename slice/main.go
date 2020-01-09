package main

import "fmt"

func main() {
	Init()
	//AssignSliceToAnother()
	//AppendSlice()
}

func Init() {
	//初始化方式
	s := make([]int, 3, 5)
	fmt.Printf("%#v,%d,%d\n", s, len(s), cap(s)) //[]int{0, 0, 0},3,5
	s2 := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	fmt.Printf("%#v,%d,%d\n", s2, len(s2), cap(s2)) //[]int{1, 2, 3, 4, 5, 6, 7, 8, 9},9,9

	//只声明不初始化，则得到slice类型的零值nil
	//len() cap() 可用于nil slice,返回0
	var s3 []int
	fmt.Printf("%#v,%d,%d\n", s3, len(s3), cap(s3)) // []int(nil),0,0

	s1 := make([]int, 2, 5)
	fmt.Printf("%#v,%d,%d\n", s1, len(s1), cap(s1)) //[]int{0, 0},2,5
	s1 = []int{1, 2}                                //等号右边的表达式创建新的切片和底层数组，s1之前指向的底层数组不会改变，但s1不再指向它
	fmt.Printf("%#v,%d,%d\n", s1, len(s1), cap(s1)) //[]int{1, 2},2,2
}

//从切片分配切片
func AssignSliceToAnother() {
	s := make([]int, 7, 10)
	for i := 0; i < 7; i++ {
		s[i] = i + 1
	}
	fmt.Printf("%#v,%d,%d\n", s, len(s), cap(s)) //[]int{1, 2, 3, 4, 5, 6, 7},7,10
	//相当于索引范围 [2,4)，这种达方式冒号分割的前两个数字分别表示原切片的索引范围
	// 新切片可以向后扩展知道底层数组的,所以新切片的容量为索引起始范围到底层数组末尾，对于 s[2:4] 即10 - 2 = 8
	s1 := s[2:4]
	fmt.Printf("%#v,%d,%d\n", s1, len(s1), cap(s1)) //[]int{3, 4},2,8
	s2 := s[:2]
	fmt.Printf("%#v,%d,%d\n", s2, len(s2), cap(s2)) //[]int{1, 2},2,10
	s3 := s[2:]
	fmt.Printf("%#v,%d,%d\n", s3, len(s3), cap(s3)) //[]int{3, 4, 5, 6, 7},5,8
	s4 := s[2:4:7]                                  // 冒号分割的第三个数字（7）表示新切片在原切片上的向后扩展的所以边界（不包含7）,这里s4能透视到的底层数组的范围为索引[2,7),所以s4容量为5
	fmt.Printf("%#v,%d,%d\n", s4, len(s4), cap(s4)) //[]int{3, 4},2,5
	s5 := s3[1:2:6]
	fmt.Printf("%#v,%d,%d\n", s5, len(s5), cap(s5)) //[]int{4},1,5
	// s6基于s4分配，虽然底层数组索引范围为[0,9],但是s5向后扩展的边界不能超过s4的边界，即s5能分配的范围为s4[0，5),所以最后一个数字的最大值为5，超过则报错
	s6 := s4[1:2:6] //panic: runtime error: slice bounds out of range [::6] with capacity 5
	fmt.Printf("%#v,%d,%d\n", s6, len(s6), cap(s6))
}

func AppendSlice() {
	// append() 的第一、第二个参数都可以是为nil的slice
	var s []int
	s = append(s, 8)
	fmt.Printf("%#v,%d,%d\n", s, len(s), cap(s)) //[]int{8},1,1

	var s1, s2 []int
	s1 = append(s1, s2...)                          //s1还是nil
	fmt.Printf("%#v,%d,%d\n", s1, len(s1), cap(s1)) //[]int(nil),0,0

	//当append()的原slice的容量不足以完成append，底层实现会新分配一个更大的底层数组，原slice的底层数组的元素会拷贝到新数组
	//如果没有分配新数组，原数组会被改变，指向原数组的所有slice也会相应改变，但是如果新分配数组，则指向原数组的所有切片不会同步改变
	s3 := make([]int, 0, 3)
	s33 := append(s3, []int{1, 2}...)
	fmt.Printf("%#v,%d,%d\n", s3, len(s3), cap(s3))    //[]int{},0,3
	fmt.Printf("%#v,%d,%d\n", s33, len(s33), cap(s33)) //[]int{1, 2},2,3

	s333 := append(s33, []int{4, 5, 6, 7}...)             //s33不变，因为检测到s33容量不够，会直接分配更大的底层数组，原数组不会被更改
	fmt.Printf("%#v,%d,%d\n", s33, len(s33), cap(s33))    //[]int{1, 2},2,3
	fmt.Printf("%#v,%d,%d\n", s333, len(s333), cap(s333)) //[]int{1, 2, 4, 5, 6, 7},6,6
}
