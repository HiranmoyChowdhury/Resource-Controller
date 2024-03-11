package handler

import "fmt"

var (
	counter   int32  = 2
	ownerName string = "RanChy36"
)

func NextLabel() string {
	counter++
	return CurrentLabel()
}
func CurrentLabel() string {
	ret := ownerName
	fmt.Println(ret)
	ret += "skdjk"
	fmt.Println(ret)
	return ret
}
