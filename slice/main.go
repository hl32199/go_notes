package main

import "fmt"

func main() {
	s := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	s2 := s[2:5]
	fmt.Println(s2, len(s2), cap(s2))
	s3 := s[2:5:7]
	fmt.Println(s3, len(s3), cap(s3))
	s3 = append(s3, 11)
	s3 = append(s3, 12)
	s3 = append(s3, 13)
	fmt.Println(s3, len(s3), cap(s3))
	fmt.Println(s2, len(s2), cap(s2))
	fmt.Println(s, len(s), cap(s))
}
