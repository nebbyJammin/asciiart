package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/nebbyJammin/asciiart/pkg/asciiart"
)

const (
	colorUsage 			= "Enables color. By default uses 4 bit color space."
	sobelUsage 			= "Enables sobel edge detection."
	boldUsage			= "Enables bold outline. Will only work if -s flag is enabled."
	downscalingUsage	= "Specifes which downscaling mode to use:\n" +
					      `    - "respect-aspect-ratio"` + "\n" +
						  `    - "ignore-aspect-ratio"` + "\n"
	aspectUsage			= "Specifies the output aspect ratio to use. Use the inverse of the aspect ratio of the terminal character you are targetting (usually the output aspect ratio will approximately be 2:1 = 2)."
	colorSpaceUsage		= "Specifies the color space to use:\n" +
							`  - "3bit" | "3"` + "\n" +
							`  - "4bit" | "4"` + "\n" +
							`  - "8bit" | "8"` + "\n" +
							`  - "24bit"| "24" | truecolor | full`+ "\n"

	widthUsage			= "Specifies the target width. May be ignored depending on the downsampling mode."
	heightUsage			= "Specifies the target height. May be ignored depending on the downsampling mode."
	richUsage			= "Alias for -c -s -b -cspace=24bit"
)

func main() {
	useColor := false
	useSobel := false
	useBoldOutline := true
	downscalingModeStr := "respect-aspect-ratio"
	aspectRatio := float64(2)
	colorSpace := "4bit"
	width := 100
	height := 100

	enableColor := func(s string) error {
		useColor = true	
		return nil
	}

	enableSobel := func(s string) error {
		useSobel = true
		return nil
	}

	enableBold := func(s string) error {
		useBoldOutline = true
		return nil
	}

	enableRich := func(s string) error {
		if err := enableBold(s); err != nil {
			return err
		}
		if err := enableSobel(s); err != nil {
			return err
		}
		if err := enableColor(s); err != nil {
			return err
		}
		colorSpace = "24bit"
		
		return nil
	}

	flag.BoolFunc("c", colorUsage, enableColor)
	flag.BoolFunc("color", "alias for -c", enableColor)

	flag.BoolFunc("s", sobelUsage, enableSobel)
	flag.BoolFunc("sobel", "alias for -s", enableSobel)

	flag.BoolFunc("b", boldUsage, enableBold)
	flag.BoolFunc("bold", "alias for -b", enableBold)

	flag.Float64Var(&aspectRatio, "a", 2, aspectUsage)
	flag.Float64Var(&aspectRatio, "aspect-ratio", 2, "alias for -a")

	flag.IntVar(&width, "w", 100, widthUsage)
	flag.IntVar(&width, "width", 100, "alias for -w")
	flag.IntVar(&height, "h", 100, heightUsage)
	flag.IntVar(&width, "height", 100, "alias for -h")

	flag.StringVar(&downscalingModeStr, "downscale-mode", "respect-aspect-ratio", downscalingUsage)

	flag.StringVar(&colorSpace, "cspace", "4bit", colorSpaceUsage)
	flag.StringVar(&colorSpace, "color-space", "4bit", "alias for -cspace")

	flag.BoolFunc("rich", richUsage, enableRich)
	flag.BoolFunc("r", richUsage, enableRich)

	// Parse flags
	flag.Parse()

	// Interpret downscaling mode string as enum value
	var dMode asciiart.DownscalingMode

	switch downscalingModeStr {
		case "respect-aspect-ratio", "respect", "wrt":
			dMode = asciiart.DownscalingModes.WithRespectToAspectRatio()
		case "ignore-aspect-ratio", "ignore", "ign":
			dMode = asciiart.DownscalingModes.IgnoreAspectRatio()
		default:
			msg := fmt.Sprintf("Got unknown downscaling mode: %s", downscalingModeStr)
			panic(msg)
	}

	// var colorMapper func(asciiart.LuminosityProvider, int, int) (int, string)
	var colorMapperOpt asciiart.AsciiOption

	switch colorSpace {
	case "3bit", "3":
		colorMapperOpt = asciiart.WithDefault3BitColorMapper()
	case "4bit", "4":
		colorMapperOpt = asciiart.WithDefault4BitColorMapper()
	case "8bit", "8":
		colorMapperOpt = asciiart.WithDefault8BitColorMapper()
	case "24bit", "24":
		colorMapperOpt = asciiart.WithDefault24BitColorMapper()
	default:
		msg := fmt.Sprintf("Got unknown color space: %s", colorSpace)
		panic(msg)
	}

	asciiconv := asciiart.New(
		asciiart.WithSobelMagSquaredThresholdNormalized(80000),
		asciiart.WithSobelLaplacianThresholdNormalized(300),
		asciiart.WithBoldedSobelOutline(useBoldOutline),
		asciiart.WithOutputAspectRatio(aspectRatio),
		asciiart.WithDownscalingMode(dMode),
		asciiart.WithColor(useColor),
		asciiart.WithSobel(useSobel),
		asciiart.WithDefaultLumosityMapper(),
		asciiart.WithDefaultEdgeMapperFactory(),
		colorMapperOpt,
	)

	args := flag.Args()
	if len(args) == 0 {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			filename := scanner.Text()
			res, err := convertAscii(asciiconv, filename, width, height)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				continue
			}

			fmt.Println(res)
		}
	} else {
		for _, arg := range args {
			res, err := convertAscii(asciiconv, arg, width, height)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				continue
			}

			fmt.Println(res)
		}
	}
}

func convertAscii(asciiconv *asciiart.AsciiConverter, path string, width, height int) (string, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("Error reading file %s: %w", f, err)
	}

	res, err := asciiconv.ConvertBytes(f, width, height)
	if err != nil {
		return "", fmt.Errorf("Error converting ascii: %s", err)
	}

	return res, nil
}
