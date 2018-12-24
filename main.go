package main

import (
	"os"
)

var Version string

func main() {
	app := MakeApp()
	_ = app.Run(os.Args)
}


