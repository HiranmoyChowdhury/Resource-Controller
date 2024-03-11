package handler

var port int32 = 30000

func GetPort() *int32 {
	ret := port
	port++
	return &ret

}
