package main

import (
	"os"
)

var Version string

func main() {
	app := MakeApp()
	app.Run(os.Args)
}
