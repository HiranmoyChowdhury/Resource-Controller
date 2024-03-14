package controller

type helper struct {
	counter          int32
	ownerName        string
	port             int32
	deploymentPrefix int32
	servicePrefix    int32
}

func (h *helper) String(n int32) string {
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

func (h *helper) GetPort() *int32 {
	ret := h.port
	h.port++
	return &ret

}

func (h *helper) NextLabel() string {
	h.counter++
	return h.CurrentLabel()
}
func (h *helper) CurrentLabel() string {
	ret := h.ownerName
	ret += h.String(h.counter)
	return ret
}

func (h *helper) ToLowerCase(s string) string {
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
