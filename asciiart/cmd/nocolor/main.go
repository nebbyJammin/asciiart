package main

import "github.com/nebbyJammin/asciiart"

func main() {
	ascii := asciiart.New(
		asciiart.WithAlwaysDownscaleToTarget(true),
		asciiart.WithColor(false),
	)

	ascii.Convert(nil, 600, 300)
}
