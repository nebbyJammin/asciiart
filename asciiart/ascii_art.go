package asciiart

import (
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"bytes"
	"image"
	"io"
	"math"
	"strings"
)

const (
	bytesPerCharReserve						= 3.5 // assume avg 3.5 bytes per pixel
	ansiAdditionalBytesReserved3Bit 					= 2 // reserve an extra 2 bytes per pixel to allow room for ANSI escape sequences

	ansiAdditionalBytesReserved4Bit 					= 5 // reserve an extra 5 bytes per pixel to allow room for ANSI escape sequences
	ansiAdditionalBytesReserved8Bit						= 8 // reserve an extra 8 bytes per pixel to allow room for ANSI escape sequences
	ansiAdditionalBytesReserved24Bit					= 16 // reserve an extra 16 bytes per pixel to allow room for ANSI escape sequences
)

/*
ColorMapper3BitOptions represents the configuration of the color mapper.
*/
type ColorMapper3BitOptions struct {
	// ColorThresholds specifies the minimum value for each channel [r, g, b] required to register as that colour.
	ColorThresholds	[3]int
	// DoReward adds extra value to 1st, 2nd, 3rd most dominant channels based on ColorRewards and DefaultReward
	DoReward		bool
	// ColorRewards adds additional value for the 1st, 2nd, 3rd strongest channels if the maximum delta between any two channels is bigger than ColorRewardMinRange.
	ColorRewards	[3]int
	// DefaultReward is added to all 3 rgb channels if the maximum delta between any two channels is smaller than ColorRewardMinRange
	DefaultReward	int
	// ColorRewardMinRange is the lower threshold for which ColorRewards is applied if the maximum delta between any two channels is bigger than this value.
	ColorRewardMinRange int
	// BlackLumUpper is the upper bound (inclusive) for luminosity, for which pixels are rendered as black
	BlackLumUpper	int
	// WhiteLumLower is the lower bound (inclusive) for luminosity, for which pixels are rendered as white
	WhiteLumLower	int
}

/*
ColorMapper4BitOptions represents the configuration of the 4 bit color mapper. It uses a very similar technique to the 3 bit equivalent.
*/
type ColorMapper4BitOptions struct {
	ColorMapper3BitOptions
	// BoldColoredLumLower is the lower bound (inclusive) for luminosity, for which colored codes will become its bold variant
	BoldColoredLumLower	int
	// BoldBlackLumLower is the lower bound (inclusive) for luminosity, for which the black code (30) will become the bold version (90)
	BoldBlackLumLower 	int
	// BoldBlackLumLower is the lower bound (inclusive) for luminosity, for which the white code (37) will become the bold version (97)
	BoldWhiteLumLower 	int
}

/*
ColorMapper8BitOptions represents the configuration of the 8 bit color mapper. It uses a different technique to 3 bit and 4 bit. It will take step arrays for each r, g, b channel as well as a grey step array (of size 3).

NOTE: 8-bit color uses 6x6x6 color cube - so the step fields give context for what colors to use.

Each array is structured like the following:
	- [lowest val, second val, step]
		- [0] is the lowest value
		- [1] is the second value
		- [2] is the additional increase in the channel per step

For example:
	- rStep: [3]int{0, 95, 40} (the default on terminals)
		- The red channel steps would be [0, 95, 135, 175, 215, 255]
		- Usually all rgb channels follow this, grey step will usually be [8, 18, 10]
	
	The generated step values represent what colours can be made on the cube.
*/
type ColorMapper8BitOptions struct {
	rStep				[3]int
	gStep				[3]int
	bStep				[3]int
	greyStep			[3]int
}

// downscalingModes is the private struct that functions as a namespace for the enum DownscalingMode
type downscalingModes struct { }

// DownscalingModes is the public instance of downscalingModes. Do not reassign this variable
var DownscalingModes = downscalingModes{}

type DownscalingMode int

/*
WithRespectToAspectRatio signals to the downscaling function to downscale the image to the desired size by following the aspect ratio declared in the AsciiConverter. It will never upscale, so in most cases, the height will be shrunk by a factor of 2, since most terminals use a 1:2 character height (and therefore, the output ratio is 2:1 = 2)
*/
func (d downscalingModes) WithRespectToAspectRatio() DownscalingMode { 
	return DownscalingMode(0) 
}

/*
IgnoreAspectRatio signals to the downscaling function to downscale the image to the desired size by ignoring the specified aspect ratio declared in the AsciiConverter. It will never upscale, and will scale to the targetWidth and targetHeight specified in Convert() and DownscaleImage() (so long as it is smaller than the src width and height).
*/
func (d downscalingModes) IgnoreAspectRatio() DownscalingMode { 
	return DownscalingMode(1) 
}

type AsciiConverter struct {
	// SobelMagnitudeSqThresholdNormalized provides the minium gMag2 value before an edge is registered as an edge. This field only has an effect if UseSobel is true. See WithSobelMagSquaredThresholdNormalized()
	SobelMagnitudeSqThresholdNormalized				float64

	// SobelLaplacianMagnitudeThreshold provides the maximum laplacian value for an edge to be considered an edge. See WithSobelLaplacianThresholdNormalized()
	SobelLaplacianThresholdNormalized				float64

	// Will use bold characters to outline edges detected by the sobel edge detection. Because of this, this only has an effect if UseSobel is true, and if the algorithm can actually detect any edges
	SobelOutlineIsBold								bool

	// OutputAspectRatio is the aspect ratio of the resulting image (char_x / char_y).
	// In most cases, a terminal character's height is twice its width. 
	// So the resulting image must be 2:1 ratio to compensate for the taller height
	OutputAspectRatio 								float64

	//DownscalingMode flags to the converter how to downscale the image before any conversion happens. By default, it will ALWAYS downscale with respect to the aspect ratio (DownscalingModes.WithRespectToAspectRatio() [0])
	DownscalingMode									DownscalingMode

	// UseSobel flags to the converter whether or not sobel edge detection should be used.
	UseSobel										bool
	
	// UseColor flags to the converter whether terminal escape sequences used to indicate colour should be used
	UseColor										bool
	// The function that converts a luminence value (0-255) to a rune
	LuminosityMapper									func(lumProv LuminosityProvider, x, y int) rune
	// The function that converts an approximate gradient to a rune
	EdgeMapperFactory								func(aspect_ratio float64) func(sobelProv SobelProvider, x, y int) rune
	ANSIColorMapper									func(lumProv LuminosityProvider, x, y int) (code_id int, fmted_code string)

	// BytesPerCharToReserve is the amount of bytes per character to reserve in the result buffer
	BytesPerCharToReserve							float64
	// AdditionalBytesPerCharColor is the amount of additional bytes per character to reserve in the result buffer if color is being used
	AdditionalBytesPerCharColor 					float64
}

type asciioption func(*AsciiConverter)

/*
defaultLuminosityProvider is the default LuminosityProvider implementation that caches luminosity data, so it does not need to be calculated again.

NOTE: Do not resize the image after constructing a defaultLuminosityProvider. They are intended to be immutable after construction.
*/
type defaultLuminosityProvider struct {
	// Stores the underlying image data. Useful for getting the color data of a char
	image.Image
	// LumData stores the raw luminosity (0-255) of each char in a 1D array
	LumData 	[]int
	width		int
	height		int
}

func makeDefaultLuminosityImage(img image.Image) defaultLuminosityProvider {
	bounds := img.Bounds()
	dx, dy := bounds.Dx(), bounds.Dy()

	return defaultLuminosityProvider{
		LumData: make([]int, dx * dy),
		Image: img,
		width: dx,
		height: dy,
	}
}

/*
LuminosityAt returns the luminosity (0-255) at some x, y pixel. However, if you need 1D iteration, you can use x as the iterating variable, and set y = 0. Alternatively, you can use LuminosityAt1D() instead. The LuminosityAt() function does not check if x and y are actually valid. Essentially under the hood it is doing.
*/
func (d defaultLuminosityProvider) LuminosityAt(x, y int) int {
	return d.LumData[x + d.width * y]
}

// LuminosityAt1D returns the luminosity (0-255) at some idx in the 1D backing array for luminosity
func (d defaultLuminosityProvider) LuminosityAt1D(idx int) int {
	return d.LumData[idx]
}

/*
LuminosityAt returns the luminosity (0-255) at some x, y pixel. For this reason, 1D iteration hack does not work for this (because we are checking the validity of x and y)
*/
func (d defaultLuminosityProvider) SafeLuminosityAt(x, y int) int {
	xSafe, ySafe := x, y
	if x < 0 {
		xSafe = 0
	} else if x >= d.width {
		xSafe = d.width - 1
	}

	if y < 0 {
		ySafe = 0
	} else if y >= d.height {
		ySafe = d.height - 1
	}

	return d.LumData[xSafe + d.width * ySafe]
}

func (d defaultLuminosityProvider) LuminositySet1D(idx int, lum int) {
	d.LumData[idx] = lum
}

func (d defaultLuminosityProvider) LuminositySet(x, y int, lum int) {
	d.LumData[x + y * d.width] = lum
}

func (d defaultLuminosityProvider) Width() int {
	return d.width
}

func (d defaultLuminosityProvider) Height() int {
	return d.height
}

/*
defaultSobelProvider is the default implementation for the SobelProvider interface. It caches Sobel gradient, magnitude squared and laplacian so that it does not need to be recalculated. Use this for edge detection
*/
type defaultSobelProvider struct {
	LuminosityProvider
	G_Grad		[]float64
	G_Mag2		[]int
	G_Laplacian	[]float64
}

func makeDefaultSobelProvider(lumProvider LuminosityProvider, gGrad []float64, gMag2 []int, gLap []float64) defaultSobelProvider {
	return defaultSobelProvider{
		LuminosityProvider: lumProvider,
		G_Grad: gGrad,
		G_Mag2: gMag2,
		G_Laplacian: gLap,
	}
}

func (d defaultSobelProvider) SobelGradAt1D(idx int) float64 {
	return d.G_Grad[idx]
}

func (d defaultSobelProvider) SobelGradAt(x, y int) float64 {
	return d.G_Grad[x + y * d.Width()]
}

func (d defaultSobelProvider) SobelMag2At1D(idx int) int {
	return d.G_Mag2[idx]
}

func (d defaultSobelProvider) SobelLaplacianAt(x, y int) float64 {
	return d.G_Laplacian[x + y * d.Width()]
}

func (d defaultSobelProvider) SobelLaplacianAt1D(idx int) float64 {
	return d.G_Laplacian[idx]
}

/*
SobelMag2At returns the sobel magnitude squared at some x, y pixel. However, if you need 1D iteration, use x as the iterating variable, and set y = 0. Alternatively, you can use SobelMag2At1D() instead. The SobelMag2At() function does not check if x and y are actually valid. Essentially under the hood it is doing:

	return d.gMag2[x + y * d.width]
*/
func (d defaultSobelProvider) SobelMag2At(x, y int) int {
	return d.G_Mag2[x + y * d.Width()]
}

/*
LuminosityProvider is the interface that stores and provides luminosity data per character
*/
type LuminosityProvider interface {
	image.Image
	LuminosityAt1D(int) int
	LuminosityAt(int, int) int
	SafeLuminosityAt(int, int) int
	LuminositySet1D(int, int)
	LuminositySet(int, int, int)
	Width() int
	Height() int
}

/*
SobelProvider is the interface that stores and provides sobel data per character. This includes the Sobel gradient, magnitude and laplacian, which can be used to detect edges
*/
type SobelProvider interface {
	image.Image
	LuminosityProvider
	SobelGradAt1D(int) float64
	SobelGradAt(int, int) float64
	SobelMag2At1D(int) int
	SobelMag2At(int, int) int
	SobelLaplacianAt(int, int) float64
	SobelLaplacianAt1D(int) float64
}

/*
NewDefault initializes an asciiart instance with default parameters.

- SobelMagnitudeThresholdNormalized: 80000
- SobelLaplacianThresholdNormalized: 300
- SobelOutlineIsBold: true
- OutputAspectRatio: 2
- DownscalingMode: DownscalingModes.WithRespectToAspectRatio() [0]
- UseColor: true
- UseSobel: true
- LuminenceMapper: <default internal luminence mapper>
- EdgeMapperFactor: <default internal edge mapper factory>
- ANSIColorMapper: <default internal 4 bit color mapper>:
	- NOTE: You can choose which color mapper to choose from with asciioption:
		- 3bit, 4bit, 8bit, 24bit
- BytesPerCharToReserve: 3.5
- AdditionalBytesPerCharColor: 2
*/
func NewDefault() *AsciiConverter {
	return &AsciiConverter {
		SobelMagnitudeSqThresholdNormalized: 80000,
		SobelLaplacianThresholdNormalized: 300,
		SobelOutlineIsBold: true,
		OutputAspectRatio: 2,
		DownscalingMode: DownscalingModes.WithRespectToAspectRatio(),
		UseColor: true,
		UseSobel: true,
		LuminosityMapper: DefaultLuminenceMapper,
		EdgeMapperFactory: DefaultEdgeMapperFactory,
		// ANSIColorMapper: defaultColorMapper(),
		ANSIColorMapper: Default4BitColorMapper(),
		BytesPerCharToReserve: bytesPerCharReserve,
		AdditionalBytesPerCharColor: ansiAdditionalBytesReserved3Bit,
	}
}

// New initializes an asciiart instance with default parameters, then applies options
func New(opts ...asciioption) *AsciiConverter {
	ascii := NewDefault()

	for _, o := range opts {
		o(ascii)
	}

	return ascii
}

/*
defaultColorMapper provides the default configuration for the 3 bit color mapper provided by this library. 99% of terminals should support at least 3 bit color space.
*/
func defaultColorMapper() func(LuminosityProvider, int, int) (int, string) {
	return Default3BitColorMapper()
}

var default3BitOpts = ColorMapper3BitOptions {
	// ColorThresholds: [3]int{130, 140, 120},
	// ColorThresholds: [3]int{110, 120, 80},
	ColorThresholds: [3]int{140, 150, 110},
	DoReward: true,
	ColorRewards: [3]int{40, 20, 10},
	DefaultReward: 24,
	ColorRewardMinRange: 20,
	BlackLumUpper: 50,
	WhiteLumLower: 200,
}

/*
defaultColorMapper provides the default configuration for the 3 bit color mapper provided by this library. 99% of terminals should support at least 3 bit color space.
*/
func Default3BitColorMapper() func(LuminosityProvider, int, int) (int, string) {
	return default3BitColorMapperFactory(default3BitOpts)
}

/*
Default4BitColorMapper provides the default configuration for the 4 bit color mapper provided by this library. 99% of terminals should support at least 4 bit color space.
*/
func Default4BitColorMapper() func(LuminosityProvider, int, int) (int, string) {
	opts := ColorMapper4BitOptions {
		ColorMapper3BitOptions: default3BitOpts,
		BoldColoredLumLower: 100,
		BoldBlackLumLower: 40,
		BoldWhiteLumLower: 240,
	}

	return default4BitColorMapperFactory(opts)
}

/*
Default8BitColorMapper provides the default configuration for the 8 bit color mapper provided by this library. 95%+ of terminals should support at least 8 bit color space.
*/
func Default8BitColorMapper() func(LuminosityProvider, int, int) (int, string) {
	opts := ColorMapper8BitOptions {
		rStep: [3]int{0, 95, 40},
		gStep: [3]int{0, 95, 40},
		bStep: [3]int{0, 95, 40},
		greyStep: [3]int{8, 18, 10},
	}

	return default8BitColorMapperFactory(opts)
}

/*
Default8BitColorMapper provides the default configuration for the 24 bit color mapper provided by this library. 95%+ of terminals should support 24 bit color space.
*/
func Default24BitColorMapper() func(LuminosityProvider, int, int) (int, string) {
	return default24BitColorMapperFactory()
}

/*
DefaultLuminenceMapper is the default implementation of a mapper func that takes a luminence value between 0 and 255, and returns a rune. It will use generic symbols commonly seen in ascii art. The list of symbols are:

	$@B%8&WM#*oahkbdpqwmZO0QLCJUYXzcvunxrjft/\\|()1{}[]?-_+~<>i!lI;:,"^`'.
*/
func DefaultLuminenceMapper(lumProv LuminosityProvider, x, y int) rune {
	const charRamp = `$@B%8&WM#*oahkbdpqwmZO0QLCJUYXzcvunxrjft/\|()1{}[]?-_+~<>i!lI;:,"^` + "`" + `'. `
	const charLen = float64(len(charRamp))

	luminence := lumProv.LuminosityAt(x, y)
	charIdx := len(charRamp) - int(float64(luminence) / 255 * (charLen - 1)) - 1

	return rune(charRamp[charIdx])
}

/*
DefaultEdgeMapperFactory is the default implementation of a factory that returns an edge mapper for some aspect ratio.

The function it returns translates a pixel/character that has been classified as an edge (based on its sobel magnitude squared), and then assigns a character that represents its curvature using the sobel gradient.
*/
func DefaultEdgeMapperFactory(aspect_ratio float64) func(SobelProvider, int, int) rune {
	type edgeGlyphStop struct {
		rune
		float64
	}

	const minGrad = float64(-60.5)
	const maxGrad = float64(60.5)

	stops := [...]edgeGlyphStop{
		{rune: '=', float64: math.Nextafter(-6, math.Inf(-1))},		// (-inf, -7)
		{rune: '\\', float64: math.Nextafter(-2, math.Inf(-1))},		// [-7, -5)
		{rune: 'l', float64: math.Nextafter(-1, math.Inf(-1))},		// [-5, -3)
		{rune: 'L', float64: math.Nextafter(-0.5, math.Inf(-1))},	// [-3, -0.5)
		{rune: '|', float64: 0.5},									// [-0.5, 0.5]
		{rune: 'J', float64: math.Nextafter(1, math.Inf(1))},		// (0.5, 3]
		{rune: 'j', float64: math.Nextafter(2, math.Inf(1))},		// (3, 5]
		{rune: '/', float64: math.Nextafter(7, math.Inf(1))},		// (5, 7]
		{rune: '=', float64: math.Inf(1)},							// (7, inf)
	}

	const precision = float64(10)

	glyphStopsLen := int((maxGrad - minGrad) * precision / aspect_ratio) // Aspect ratio of 2 will half the gradients
	glyphStops := make([]rune, glyphStopsLen)

	idxOffset := int(minGrad * precision / aspect_ratio) // The amount we have to shift the idx to get the gradient
	currStop := 0
	// count := 0
	for i := range glyphStops {
		thresh := stops[currStop].float64 * precision / aspect_ratio
		for float64(i + idxOffset) > thresh {
			// fmt.Println("Got", count, stops[currStop].rune)
			// count = 0

			// fmt.Println("skipping", string(stops[currStop].rune), "with inferred grad", float64(i - idxOffset), "comparing with expected", stops[currStop].float64)

			currStop++
			thresh = stops[currStop].float64 * precision / aspect_ratio
		}

		// count++
		glyphStops[i] = stops[currStop].rune
	}

	// fmt.Println(string(glyphStops))

	return func(sobelProv SobelProvider, x, y int) rune {
		grad := sobelProv.SobelGradAt(x, y)
		gradIdx := min(glyphStopsLen - 1, max(0, int(grad * precision) - idxOffset))
		// fmt.Println("Got gradient", grad, (grad * aspect_ratio),  "=", gradIdx, string(glyphStops[gradIdx]))
		return glyphStops[gradIdx]
	}
}

/*
ConvertReader takes an io.Reader that can read the bytes of an image. Image formats supported are jpeg, png, gif. If you want to support more formats, initialize the decoder package at the top of any of your go files:

import (
	... <other imports>

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	...
)

ConvertReader uses image.Decode() under the hood, so it is important to register file formats so the image module knows how to decode the bytes.
*/
func (a *AsciiConverter) ConvertReader(r io.Reader, targetWidth, targetHeight int) (string, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return "", err
	}

	return a.Convert(img, targetWidth, targetHeight), nil
}

/*
ConvertBytes takes a byte slice representing an image. Image formats supported are jpeg, png, gif. If you want to support more formats, initialize the decoder package at the top of any of your go files:

import (
	... <other imports>

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	_ "mycustomdecoder/mycustomformat" // Here is your custom file format
	
	...
)

ConvertBytes calls ConvertReader() under the hood which uses image.Decode(), so it is important to register file formats so the image module knows how to decode the bytes.
*/
func (a *AsciiConverter) ConvertBytes(b []byte, targetWidth, targetHeight int) (string, error) {
	return a.ConvertReader(bytes.NewReader(b), targetWidth, targetHeight)
}

/*
DownscaleImage downscales the src image to some targetWidth/targetHeight, however, does it in different ways depending on the DownscalingMode. 

In DownscalingModes.WithRespectToAspectRatio() mode:
	- Will never downscale below targetWidth or targetHeight.
	- Instead uses the aspect ratio to shrink either the targetWidth or targetHeight to achieve the configured OutputAspectRatio.
	- In the common case of OutputAspectRatio=2 (the most common for terminals, because width of the output is twice the height to account for 1:2 character dimensions), the targetWidth will be used, and the targetHeight param will be ignored. Instead the height will be proportional (with respect to the aspect ratio) to the width.
	- Conversely, if you have OutputAspectRatio=0.5 (this will probably never happen, where your characters are twice as wide as they are tall, so the output is twice as skinny), the targetHeight will be used, and the targetWidth param will be ignored. Instead the width will be proportional (with respect to the aspect ratio) to the height.

In DownscalingModes.IgnoreAspectRatio() mode:
	- The function simply downscales forcibly to the specified targetWidth and targetHeight.
	- As a result, you are responsible for dealing with the aspect ratio (usually beforehand, so any cropping/image manipulation needs to be done before passing into this func or Convert())

Alternatively, if you want to downscale directly to the targetWidth/targetHeight, set the DownscalingMode = to DownscalingModes.IgnoreAspectRatio
That will signal the function to always downscale to the target resolution 

Returns the downscaled image, and the effective aspect ratio. The effective aspect ratio is should be roughly equal to the original aspect ratio, but may differ because of integer clamping. Use the effective aspect ratio to adjust Sobel thresholds or gradient correction, since the sampling grid may differ slightly from OutputAspectRatio due to integer rounding.
*/
func (a *AsciiConverter) DownscaleImage(src image.Image, targetWidth, targetHeight int) (image.Image, float64) {

	// Check if we need to do anything
	if a.OutputAspectRatio == 1 && a.DownscalingMode == DownscalingModes.WithRespectToAspectRatio() {
		return src, a.OutputAspectRatio
	}

	var newWidth, newHeight int
	srcBounds := src.Bounds()
	srcWidth, srcHeight := srcBounds.Dx(), srcBounds.Dy()

	// NOTE: We will never upscale width or height. Instead, downscale the opposing axis.
	switch a.DownscalingMode {
		case DownscalingModes.WithRespectToAspectRatio():
			if a.OutputAspectRatio >= 1 {
				// aspect ratio >= 1, so downscale directly to the width, then scale height accordingly
				newWidth = targetWidth
				newHeight = int(float64(targetWidth) / a.OutputAspectRatio)
			} else {
				// aspect ratio < 1, so downscale directly to the height, then scale width accordingly
				newWidth = int(float64(targetHeight) * a.OutputAspectRatio)
				newHeight = targetHeight
			}
		case DownscalingModes.IgnoreAspectRatio():
			newWidth = targetWidth	
			newHeight = targetHeight

			// old code with scale factor:

			// if a.OutputAspectRatio >= 1 {
				// // scale width down by scale factor
				// newWidth = max(int(float64(srcWidth) / a.DownscaleFactor), targetWidth)
				// // scale height accordingly with respect to the width and aspect ratio
				// newHeight = int(float64(newWidth) / a.OutputAspectRatio)
			// } else {
				// // scale height down by scale factor
				// newHeight = max(int(float64(srcHeight) / a.DownscaleFactor), targetHeight)
				// // scale width accordingly with respect to the height and aspect ratio
				// newWidth = int(float64(newHeight) * a.OutputAspectRatio)
			// }
		default:
			msg := fmt.Sprintf("Unknown downscaling mode provided: %d", a.DownscalingMode)
			panic(msg)
	}

	if newWidth == 0 {
		panic("Downscaled width of 0 is invalid. Set a valid targetWidth")
	}

	if newHeight == 0 {
		panic("Downscaled height of 0 is undefined behaviour. Set a valid targetHeight")
	}
	
	downscaledImg := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	
	// Write pixels to the downscaled image
	for x := range newWidth {
		for y := range newHeight {
			srcX := int(float64(x) * float64(srcWidth) / float64(newWidth))
			srcY := int(float64(y) * float64(srcHeight) / float64(newHeight))

			c := src.At(srcX, srcY)

			downscaledImg.Set(x, y, c)
		}
	}

	return downscaledImg, float64(newWidth) / float64(newHeight)
}

/*
MapLuminosity returns the default implementation of LuminosityProvider from an image by precalculating all luminosity values and storing it.
*/
func (a *AsciiConverter) MapLuminosity(img image.Image) defaultLuminosityProvider {
	lumImg := makeDefaultLuminosityImage(img)
	
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	for x := range width {
		for y := range height {
			r, g, b, a := img.At(x, y).RGBA()
			r8, g8, b8, a8 := r >> 8, g >> 8, b >> 8, a >> 8
			// Lum approximation. Also scale the luminosity based on the alpha channel
			lum := int((r8 * 2126 + g8 * 7152 + b8 * 722) / 10000 * a8 / 255)
			lumImg.LuminositySet(x, y, lum)
		}
	}

	return lumImg
}

func computeGrad(x float64, y float64) float64 {
	if x == 0 {
		if y > 0 {
			return math.Inf(1)
		} else {
			return math.Inf(-1)
		}
	} else {
		return y / x
	}
}

func applySobelCentralPixel(lumImg LuminosityProvider, gGrad []float64, gMag2 []int, gLap []float64, x, y int) {
	/*
		
	Sobel Value for any character can be decomposed into a kernel for the dx and dy components as defined by the following:

			| -1  0 +1 |
	gx = 	| -2  0 +2 | * A
			| -1  0 +1 |

			| -1 -2 -1 |
	gy = 	|  0  0  0 | * A
			| +1 +2 +1 |

	<-- Magnitude -->

	From there, we can compute the magnitude, which is a scalar that quantifies how much change in brightness there is from that character to its neighbours.

	|G| = sqrt(gx^2 + gy^2)

	However, it is inefficient to square root for every character (potentially over hundreds of thousands of characters)
	Instead we will just compute and store |G|^2, and threshold based on that.

	|G|^2 = gx * gx + gy * gy

	<-- Gradient -->

	Sobel gradient represents the direction in which the change in brightness happened.
	- Positive gradient means the color change happens from bottom left to top right.
	- Negative gradient means the color change happens from top left to bottom right.
	- An approximately 0 gradient represents a color change that happened from left to right.
	- Infinite gradient means the color change happened from top to bottom (or vice versa)

	grad = gy/gx

	<-- Sobel Laplacian (Second Derivative) -->

	See https://en.wikipedia.org/wiki/Sobel_operator for more information
	*/

	idx := x + y * lumImg.Width()

	gx := -1 * lumImg.LuminosityAt(x-1,y-1) +
	+1 * lumImg.LuminosityAt(x+1,y-1) +
	-2 * lumImg.LuminosityAt(x-1,y) +
	+2 * lumImg.LuminosityAt(x+1,y) +
	-1 * lumImg.LuminosityAt(x-1,y+1) +
	+1 * lumImg.LuminosityAt(x+1,y+1)

	gy := -1 * lumImg.LuminosityAt(x-1,y-1) +
	-2 * lumImg.LuminosityAt(x,y-1) +
	-1 * lumImg.LuminosityAt(x+1,y-1) +
	+1 * lumImg.LuminosityAt(x-1,y+1) +
	+2 * lumImg.LuminosityAt(x,y+1) +
	+1 * lumImg.LuminosityAt(x+1,y+1)

	// Normally, we would have to scale the gMag2 to account for the aspect ratio.
	cur_gMag2 := gx * gx + gy * gy

	gMag2[idx] = cur_gMag2
	// This gradient is not normalised. Normally you would multiply by dX / dY to account for it.
	// Instead during lum->char translations, we will multiply the grad thresholds by dY/dX to be more efficient
	gGrad[idx] = computeGrad(float64(gx), float64(gy))

	invAspectRatio := lumImg.Height() / lumImg.Width()

	l := +(invAspectRatio) * lumImg.LuminosityAt(x, y-1) +
		+1 * lumImg.LuminosityAt(x-1,y) + 
		-2 * (2 + 1 * invAspectRatio) * lumImg.LuminosityAt(x,y) +
		+1 * lumImg.LuminosityAt(x+1,y) +
		+(invAspectRatio) * lumImg.LuminosityAt(x,y+1)

	gLap[idx] = float64(l)
}

func applySobelPixelSafely(lumImg LuminosityProvider, gGrad []float64, gMag2 []int, gLap []float64, x, y int) {
	gx := -1 * lumImg.SafeLuminosityAt(x-1,y-1) +
	+1 * lumImg.SafeLuminosityAt(x+1,y-1) +
	-2 * lumImg.SafeLuminosityAt(x-1,y) +
	+2 * lumImg.SafeLuminosityAt(x+1,y) +
	-1 * lumImg.SafeLuminosityAt(x-1,y+1) +
	+1 * lumImg.SafeLuminosityAt(x+1,y+1)

	gy := -1 * lumImg.SafeLuminosityAt(x-1,y-1) +
	-2 * lumImg.SafeLuminosityAt(x,y-1) +
	-1 * lumImg.SafeLuminosityAt(x+1,y-1) +
	+1 * lumImg.SafeLuminosityAt(x-1,y+1) +
	+2 * lumImg.SafeLuminosityAt(x,y+1) +
	+1 * lumImg.SafeLuminosityAt(x+1,y+1)

	// Normally, we would have to scale the gMag2 to account for the aspect ratio.
	cur_gMag2 := gx * gx + gy * gy
	idx := x + y * lumImg.Width()

	gMag2[idx] = cur_gMag2
	// This gradient is not normalised. Normally you would multiply by dX / dY to account for it.
	// Instead during lum->char translations, we will multiply the grad thresholds by dY/dX to be more efficient
	gGrad[idx] = computeGrad(float64(gx), float64(gy))

	invAspectRatio := lumImg.Height() / lumImg.Width()

	l := +(invAspectRatio) * lumImg.SafeLuminosityAt(x, y-1) +
		+1 * lumImg.SafeLuminosityAt(x-1,y) + 
		-2 * (2 + 1 * invAspectRatio) * lumImg.SafeLuminosityAt(x,y) +
		+1 * lumImg.SafeLuminosityAt(x+1,y) +
		+(invAspectRatio) * lumImg.SafeLuminosityAt(x,y+1)

	gLap[idx] = float64(l)
}

/*
ApplySobel returns the defaultSobelProvider implementation of SobelProvider from a luminosity provider
*/
func (a *AsciiConverter) ApplySobel(lumImg LuminosityProvider) defaultSobelProvider {	
	gWidth := lumImg.Width()
	gHeight := lumImg.Height()

	gLen := gWidth * gHeight
	gMag2 := make([]int, gLen)
	gGrad := make([]float64, gLen)
	gLap := make([]float64, gLen)

	// Calculate G
	for y := 1; y < gHeight - 1; y++ {
		for x := 1; x < gWidth - 1; x++ {
			applySobelCentralPixel(lumImg, gGrad, gMag2, gLap, x, y)
		}
	}

	// Apply left/right sides
	for x := range gWidth {
		applySobelPixelSafely(lumImg, gGrad, gMag2, gLap, x, 0)
		applySobelPixelSafely(lumImg, gGrad, gMag2, gLap, x, gHeight - 1)
	}

	// Apply bottom/top (skipping corners that we have already done)
	for y := 1; y < gHeight - 1; y++ {
		applySobelPixelSafely(lumImg, gGrad, gMag2, gLap, 0, y)
		applySobelPixelSafely(lumImg, gGrad, gMag2, gLap, gWidth-1, y)
	}

	return makeDefaultSobelProvider(lumImg, gGrad, gMag2, gLap)
}

/*
ASCIIGenWithSobel converts a SobelProvider to ascii string. If you are not interested in making custom ascii generators, see Convert(), ConvertBytes() and ConvertReader()
*/
func (a *AsciiConverter) ASCIIGenWithSobel(sobelProv SobelProvider, aspect_ratio float64) string {
	adjustedGMag2Threshold := int(a.SobelMagnitudeSqThresholdNormalized * (aspect_ratio * aspect_ratio))

	width, height := sobelProv.Width(), sobelProv.Height()
	// numPixels := width * height

	edgeMapper := a.EdgeMapperFactory(aspect_ratio)

	var bufferSize int
	if a.UseColor {
		// In most cases, we will overallocate by a few hundred bytes to ensure there is no reallocation of the buffer
		// This is because it cannot be known how much room should be left for the colour ANSI escape sequences
		bufferSize = int((bytesPerCharReserve + ansiAdditionalBytesReserved3Bit) * float64(width + 1) * float64(height)) // width + 1 because leave a byte for the new line byte
	} else {
		bufferSize = int(bytesPerCharReserve * float64(width + 1) * float64(height)) // width + 1 because leave a byte for the new line
	}

	var asciiBuilder strings.Builder
	asciiBuilder.Grow(bufferSize)
	var prevColor int = -1
	var prevWasBold bool = false

	// Reset everything before we write
	asciiBuilder.WriteString("\x1b[0m")

	for y := range height {
		for x := range width {
			code, escapeStr := a.ANSIColorMapper(sobelProv, x, y)
			if code != prevColor {
				prevColor = code

				asciiBuilder.WriteString(escapeStr)
			}

			if sobelProv.SobelMag2At(x, y) >= adjustedGMag2Threshold &&
				math.Abs(sobelProv.SobelLaplacianAt(x, y)) <= a.SobelLaplacianThresholdNormalized {
				if a.SobelOutlineIsBold && !prevWasBold {
					prevWasBold = true
					asciiBuilder.WriteString("\x1b[1m") // Make bold
				}

				asciiBuilder.WriteRune(edgeMapper(sobelProv, x, y))
			} else {
				if prevWasBold {
					prevWasBold = false
					asciiBuilder.WriteString("\x1b[22m") // Reset bold
				}

				asciiBuilder.WriteRune(a.LuminosityMapper(sobelProv, x, y))
			}
		}
		asciiBuilder.WriteRune('\n')
	}

	asciiBuilder.WriteString("\x1b[0m")
	
	return asciiBuilder.String()
}

/*
ASCIIGen takes a LuminosityProvider and generates an ascii string from it. If you are not interested in making custom ascii generators, see Convert(), ConvertBytes() and ConvertReader()
*/
func (a *AsciiConverter) ASCIIGen(lumProv LuminosityProvider, aspect_ratio float64) string {
	width, height := lumProv.Width(), lumProv.Height()

	var bufferSize int
	if a.UseColor {
		// In most cases, we will overallocate by a few hundred bytes to ensure there is no reallocation of the buffer
		// This is because it cannot be known how much room should be left for the colour ANSI escape sequences
		bufferSize = int((a.BytesPerCharToReserve + a.AdditionalBytesPerCharColor) * float64(width + 1) * float64(height)) // width + 1 because leave a byte for the new line byte
	} else {
		bufferSize = int(a.BytesPerCharToReserve * float64(width + 1) * float64(height)) // width + 1 because leave a byte for the new line
	}

	var asciiBuilder strings.Builder
	asciiBuilder.Grow(bufferSize)

	if a.UseColor {
		var prevColor int = -1

		for y := range height {
			for x := range width {
				code, escapeStr := a.ANSIColorMapper(lumProv, x, y)
				if code != prevColor {
					prevColor = code

					asciiBuilder.WriteString(escapeStr)
				}

				asciiBuilder.WriteRune(a.LuminosityMapper(lumProv, x, y))
			}
			asciiBuilder.WriteRune('\n')
		}

		asciiBuilder.WriteString("\x1b[0m")

	} else {

		for y := range height {
			for x := range width {
				asciiBuilder.WriteRune(a.LuminosityMapper(lumProv, x, y))
			}
			asciiBuilder.WriteRune('\n')
		}

		asciiBuilder.WriteString("\x1b[0m")
	}
	
	return asciiBuilder.String()
}

/*
Convert takes an image and generates an ascii art string with targetWidth and targetHeight parameters. 

However, if targetWidth and targetHeight do not follow the OutputAspectRatio, then one of targetWidth and targetHeight will be ignored by default (usually height if you are using OutputAspectRatio = 2 which is standard).

To ignore this behaviour and always convert to target width and height, specify DownscalingMode to be equal to DownscalingModes.IgnoreAspectRatio
*/
func (a *AsciiConverter) Convert(img image.Image, targetWidth, targetHeight int) string {
	var effectiveAspectRatio float64
	img, effectiveAspectRatio = a.DownscaleImage(img, targetWidth, targetHeight)
	lumImg := a.MapLuminosity(img)

	if a.UseSobel {
		sobelImg := a.ApplySobel(lumImg)

		return a.ASCIIGenWithSobel(sobelImg, effectiveAspectRatio)
	}

	return a.ASCIIGen(lumImg, effectiveAspectRatio)
}
