package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/nebbyJammin/asciiart"
)

func main() {
	ascii := asciiart.New(
		// asciiart.WithAlwaysDownscaleToTarget(true), // Downscaling before to target size
		asciiart.WithSobel(false), // Not using sobel
		asciiart.WithColor(false), // Not using color
	)

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	pathToImages := filepath.Join(filepath.Dir(wd), "asciiart_cmd_images")

	err = filepath.Walk(pathToImages, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return err
		}

		fmt.Printf("Image: %s\n", info.Name())

		f, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}

		start := time.Now()

		asciiStr, err := ascii.ConvertBytes(f, 100, 100)
		if err != nil {
			fmt.Printf("Error converting to ascii: %s\n", err)
			panic(err)
		}

		timeTaken := time.Since(start)

		fmt.Println(asciiStr)
		fmt.Println()

		totalTimeTaken := time.Since(start)
		
		conversionMs := timeTaken.Milliseconds()
		totalMs := totalTimeTaken.Milliseconds()
		fmt.Printf("Conversion took %dms\n", conversionMs)
		fmt.Printf("Printing and conversion took %dms\n", totalMs)

		return nil
	})

	if err != nil {
		fmt.Println(err)
	}
}
