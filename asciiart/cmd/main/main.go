package main

import (
	"flag"
)

const (
	colorUsage = "Enables color. By default uses 4 bit color space"
)

func main() {
	useColor := false

	enableColor := func(s string) error {
		useColor = true	
		return nil
	}

	flag.BoolFunc("c", colorUsage, enableColor)
	flag.BoolFunc("color", colorUsage, enableColor)
	flag.BoolFunc("enableColor", colorUsage, enableColor)


}
