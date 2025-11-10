package main

import (
	"fmt"

	"github.com/nebbyJammin/asciiart/pkg/asciiart"
	utils "github.com/nebbyJammin/asciiart/cmd/internal/cmd_utils"
)

func main() {
	ascii := asciiart.New(
		// asciiart.WithAlwaysDownscaleToTarget(true), // Downscaling before to target size
		asciiart.WithSobel(true), // Using sobel
		asciiart.WithColor(true), // Color
		asciiart.WithDefault24BitColorMapper(), // Use 8 bit color mapping
	)

	err := utils.ConvertImages(ascii)
	if err != nil {
		fmt.Println(err)
	}
}


