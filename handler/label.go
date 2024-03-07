package handler

var (
	counter   int32  = 0
	ownerName string = "RanChy36"
)

func NextLabel() string {
	counter++
	return CurrentLabel()
}
func CurrentLabel() string {
	return ownerName + string(counter)
}
