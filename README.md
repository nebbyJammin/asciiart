### ascii-art - an image to ascii converter with support for color and edge detection
---

An image to ascii art converter aimed at terminals.

### Installation

To install the **API**:
```bash
go get github.com/nebbyJammin/asciiart@latest
```

To install the **command line tool**:
```bash
go install github.com/nebbyJammin/asciiart/cmd/asciiart@latest
```

### Usage

Full repository documentation can be found [here](https://pkg.go.dev/github.com/nebbyJammin/asciiart).

#### API Usage

See [documentation](https://pkg.go.dev/github.com/nebbyJammin/asciiart/pkg/asciiart) for **API** usage.

To start, you may do:

```go
api := asciiart.New()
```

Then use the `Convert()` or similar API:

```go
res, err := api.Convert(<your_image>, 100, 100) // Generates ascii with bounding box of 100x100 characters (in reality, it will probably be smaller because of original image aspect ratio)
if err != nil {
    // handle your error
}
```

For further usage, see the example in `main.go`.

#### Command Line Usage

To convert a image on the filesystem to an asciiart output in the terminal:

```asciiart [flags] file```golang

Below is a list of flags:
- `-a | -aspect-ratio `: Specifies the output aspect ratio to use. Use the inverse of the aspect ratio of the terminal character you are targetting (usually the output aspect ratio will approximately be 2:1 = 2) (default: 2)
- `-b | -bold `: Enables bold outline. Will only work if -s flag is enabled (disabled by default)
- `-cspace | -color-space`: Specifies the color space to use (default: `none`):
	+ `0bit | 0 | none | grey | greyscale | gray | grayscale`: No color
	+ `3bit | 3`: 3 bit color space. Supported by 99% of terminals
    + `4bit | 4`: 4 bit color space. Supported by 99% of terminals
    + `8bit | 8`: 8 bit color space. Supported by 95% of terminals
    + `24bit | 24`: 24 bit color space. Supported by 95% of terminals
- `-downscale-mode`: Specifies which downscaling mode to use (default: `respect-aspect-ratio`):
    + `respect-aspect-ratio`
    + `ignore-aspect-ratio`
- `-h | -height`: Specifies the target height. May be ignored depending on the downsampling mode. (default 100)
- `-r | -rich`: Alias for `-s -b -cspace=24bit`
- `-s | -sobel`: Enables sobel edge detection
- `-w | -width`: Specifies the target width. May be ignored depending on the downsampling mode. (default 100)
