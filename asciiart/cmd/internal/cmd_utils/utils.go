package utils

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/nebbyJammin/asciiart"
)

const (
	width = 100
	height = 100
)

func ConvertImages(a *asciiart.AsciiConverter) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
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
			return err
		}

		start := time.Now()

		asciiStr, err := a.ConvertBytes(f, width, height)
		if err != nil {
			fmt.Printf("Error converting to ascii: %s\n", err)
			return err
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
		return err
	}

	return nil
}
