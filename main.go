package main

import (
	"os"
	"strings"
)

var Version string

func main() {
	app := MakeApp()
	app.Run(os.Args)
}


func ifempty(a, b string) string {
	a = strings.TrimSpace(a)
	if len(a) == 0 {
		return b
	}
	return a
}