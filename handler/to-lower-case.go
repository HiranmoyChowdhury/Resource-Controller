package handler

func ToLowerCase(s string) string {
	var result string = "x"
	for _, char := range s {
		if char >= 'A' && char <= 'Z' {
			result += string(char + 32)
		} else if char <= 'a' && char >= 'z' {
			continue
		} else {
			result += string(char)
		}
	}
	return result
}
