package main

import (
	"github.com/HiranmoyChowdhury/ResourceController/controller"
	_ "k8s.io/code-generator"
)

func main() {
	controller.Start()
}
