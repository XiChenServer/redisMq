package test

import (
	"fmt"
	"testing"
)

const (
	x = iota
	_
	y
	z = "zz"
	k
	p = iota
)

func Test_iota(t *testing.T) {
	fmt.Println(x, y, z, k, p)
}

func hello(num ...int) {
	num[0] = 18
	fmt.Println(num[0], num[1], num[2])
}

func Test_bian(t *testing.T) {
	i := []int{5, 6, 7}
	hello(i...)
	fmt.Println(i[0], i[1], i[2])
}
