package main

import (
	"github.com/HiranmoyChowdhury/ResourceController/handler"
	_ "k8s.io/code-generator"
)

func main() {
	handler.Start()
}
