package controller

var (
	counter          int32  = 0
	ownerName        string = "RanChy36"
	port             int32  = 30000
	deploymentPrefix int32  = 0
	servicePrefix    int32  = 0
)

func String(n int32) string {
	buf := [11]byte{}
	pos := len(buf)
	i := int64(n)
	signed := i < 0
	if signed {
		i = -i
	}
	for {
		pos--
		buf[pos], i = '0'+byte(i%10), i/10
		if i == 0 {
			if signed {
				pos--
				buf[pos] = '-'
			}
			return string(buf[pos:])
		}
	}
}

func GetPort() *int32 {
	ret := port
	port++
	return &ret

}

func NextLabel() string {
	counter++
	return CurrentLabel()
}
func CurrentLabel() string {
	ret := ownerName
	ret += String(counter)
	return ret
}

func ToLowerCase(s string) string {
	var result string = ""
	for _, char := range s {
		if char >= 'A' && char <= 'Z' {
			result += string(char + 32)
		} else if char <= 'a' && char >= 'z' {
			continue
		} else {
			result += string(char)
		}
	}
	return result + "-"
}
