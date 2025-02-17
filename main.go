package main

import (
	"sbox/sandbox"
)

func main() {
	info, err := sandbox.FetchSystemInfo()
	if err != nil {
		panic(err)
	}
	println("Hello, world!")
	println("You're using " + info.Base + "!")
}
