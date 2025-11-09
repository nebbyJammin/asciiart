package asciiart

import (
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

type ColorMapper3BitOptions struct {
	// ColorThresholds specifies the minimum value for each channel [r, g, b] required to register as that colour.
	ColorThresholds	[3]int
	DoReward		bool
	// ColorRewards adds additional value for the 1st, 2nd, 3rd strongest channels if the maximum delta between any two channels is bigger than ColorRewardMinRange.
	ColorRewards	[3]int
	DefaultReward	int
	// ColorRewardMinRange is the lower threshold for which ColorRewards is applied if the maximum delta between any two channels is bigger than this value.
	ColorRewardMinRange int
	DoBlackThreshold bool
	DoWhiteThreshold bool
	// BlackLumUpper is the upper bound (inclusive) for luminosity, for which pixels are rendered as black
	BlackLumUpper	int
	// WhiteLumLower is the lower bound (inclusive) for luminosity, for which pixels are rendered as white
	WhiteLumLower	int
}

type ColorMapper4BitOptions struct {
	ColorMapper3BitOptions
	// BoldColoredLumLower is the lower bound (inclusive) for luminosity, for which colored codes will become its bold variant
	BoldColoredLumLower	int
	// BoldBlackLumLower is the lower bound (inclusive) for luminosity, for which the black code (30) will become the bold version (90)
	BoldBlackLumLower 	int
	BoldWhiteLumLower 	int
}

type ColorMapper8BitOptions struct {
	rStep				[3]int
	gStep				[3]int
	bStep				[3]int
	greyStep			[3]int
}

type ColorMapper24BitOptions struct {

}

type AsciiConverter struct {
	/* 
	DownscaleFactor is the scale factor at which the image is downsampled using nearest neighbour sampling
	without going below targetWidth and targetHeight when calling Convert().

	Any downscale factor < 1 will be interpreted (and potentially updated) to 1. This is because upscaling is not allowed.
	*/
	DownscaleFactor				float64
	// SobelMagnitudeThreshold provides the gMag2 value threshold before an edge is registered as an edge. This field only has an effect if UseSobel is true.
	SobelMagnitudeThreshold				float64
	
	// LaplacianMagnitudeThreshold provides the maximum laplacian value for an edge to be considered an edge.
	LaplacianMagnitudeThreshold			int

	// OutputAspectRatio is the aspect ratio of the resulting image (char_x / char_y).
	// In most cases, a terminal character's height is twice its width. 
	// So the resulting image must be 2:1 ratio to compensate for the taller height
	OutputAspectRatio 			float64
	// AlwaysDownscaleToTarget will ignore aspect ratio and forceably downscale the targetWidth, targetHeight. It is recommended to set this to false, and just configure the AspectRatio normally (default=2, and in most cases, it will work fine)
	AlwaysDownscaleToTarget			bool
	// UseSobel flags to the converter whether or not sobel edge detection should be used.
	UseSobel					bool
	// UseColor flags to the converter whether terminal escape sequences used to indicate colour should be used
	UseColor					bool
	// The function that converts a luminence value (0-255) to a rune
	LuminenceMapper				func(lumProv LuminosityProvider, x, y int) rune
	// The function that converts an approximate gradient to a rune
	EdgeMapperFactory			func(aspect_ratio float64) func(sobelProv SobelProvider, x, y int) rune
	ANSIColorMapper				func(lumProv LuminosityProvider, x, y int) (code_id int, fmted_code string)

	// BytesPerCharToReserve is the amount of bytes per character to reserve in the result buffer
	BytesPerCharToReserve		float64
	// AdditionalBytesPerCharColor is the amount of additional bytes per character to reserve in the result buffer if color is being used
	AdditionalBytesPerCharColor float64
}

type asciioption func(*AsciiConverter)

type defaultLuminosityProvider struct {
	image.Image
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
LuminosityAt returns the luminosity (0-255) at some x, y pixel. However, if you need 1D iteration, use x as the iterating variable, and set y = 0. The LuminosityAt() function does not check if x and y are actually valid. Essentially under the hood it is doing:

	return d.LumData[x + y * d.width]
*/
func (d defaultLuminosityProvider) LuminosityAt(x, y int) int {
	return d.LumData[x + d.width * y]
}

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

type defaultSobelProvider struct {
	LuminosityProvider
	G_Grad		[]float64
	G_Mag2		[]int
	G_Laplacian	[]int
}

func makeDefaultSobelProvider(lumProvider LuminosityProvider, gGrad []float64, gMag2 []int, gLap []int) defaultSobelProvider {
	return defaultSobelProvider{
		LuminosityProvider: lumProvider,
		G_Grad: gGrad,
		G_Mag2: gMag2,
		G_Laplacian: gLap,
	}
}

func (d defaultSobelProvider) SobelEdgeDetected(x, y int, threshold int) bool {
	return d.SobelMag2At(x, y) >= threshold
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

func (d defaultSobelProvider) SobelLaplacianAt(x, y int) int {
	return d.G_Laplacian[x + y * d.Width()]
}

func (d defaultSobelProvider) SobelLaplacianAt1D(idx int) int {
	return d.G_Laplacian[idx]
}

/*
SobelMag2At returns the sobel magnitude squared at some x, y pixel. However, if you need 1D iteration, use x as the iterating variable, and set y = 0. The SobelMag2At() function does not check if x and y are actually valid. Essentially under the hood it is doing:

	return d.gMag2[x + y * d.width]
*/
func (d defaultSobelProvider) SobelMag2At(x, y int) int {
	return d.G_Mag2[x + y * d.Width()]
}

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

type SobelProvider interface {
	image.Image
	LuminosityProvider
	SobelEdgeDetected(int, int, int) bool
	SobelGradAt1D(int) float64
	SobelGradAt(int, int) float64
	SobelMag2At1D(int) int
	SobelMag2At(int, int) int
	SobelLaplacianAt(int, int) int
	SobelLaplacianAt1D(int) int
}

//TODO: Update the docs
/*
NewDefault initialises an asciiart instance with default parameters.

	- DownscaleFactor: 1
	- EdgeStrength: 1
	- AspectRatio: 2
	- UseSobel: true
	- DefaultLuminenceToCharMapping

*/
func NewDefault() *AsciiConverter {
	return &AsciiConverter {
		DownscaleFactor: 1, // TODO: remove this
		SobelMagnitudeThreshold: 100000,
		OutputAspectRatio: 2,
		AlwaysDownscaleToTarget: true, // TODO: Change this to an enum, 0: Use downscale factor, 1: Downscale to target wrt aspect ratio, 2: Downscale to target irrespective of aspect ratio
		UseColor: true,
		UseSobel: true,
		LuminenceMapper: defaultLuminenceMapper,
		EdgeMapperFactory: defaultEdgeMapperFactory,
		// ANSIColorMapper: defaultColorMapper(),
		ANSIColorMapper: Default4BitColorMapper(),
		BytesPerCharToReserve: bytesPerCharReserve,
		AdditionalBytesPerCharColor: ansiAdditionalBytesReserved3Bit,
	}
}

func New(opts ...asciioption) *AsciiConverter {
	ascii := NewDefault()

	for _, o := range opts {
		o(ascii)
	}

	return ascii
}

func WithDownscaleFactor(factor float64) asciioption {
	return func(a *AsciiConverter) {
		a.DownscaleFactor = factor
	}
}

func WithEdgeStrength(strength float64) asciioption {
	return func(a *AsciiConverter) {
		a.SobelMagnitudeThreshold = strength
	}
}

func WithAspectRatio(ratio float64) asciioption {
	return func(a *AsciiConverter) {
		a.OutputAspectRatio = ratio
	}
}

func WithAlwaysDownscaleToTarget(ignore bool) asciioption {
	return func(a *AsciiConverter) {
		a.AlwaysDownscaleToTarget = ignore
	}
}

func WithSobel(useSobel bool) asciioption {
	return func(a *AsciiConverter) {
		a.UseSobel = useSobel
	}
}

func WithColor(useColor bool) asciioption {
	return func(a *AsciiConverter) {
		a.UseColor = useColor
	}
}

		// ANSIColorMapper: default4BitColorMapperFactory(
			// ColorMapper4BitOptions{
				// ColorMapperOptions: ColorMapperOptions{
					// ColorAdd: [3]int{50, 50, 50},
					// ColorScale: [3]float64{1.1, 1.1, 1.1},
				// },
			// },
		// ),

func WithLuminenceMapper(
	lumMapper func(lumProv LuminosityProvider, x, y int) rune,
) asciioption {
	return func(a *AsciiConverter) {
		a.LuminenceMapper = lumMapper
	}
}

func WithDefaultLuminenceMapper() asciioption {
	return WithLuminenceMapper(defaultLuminenceMapper)
}

func WithEdgeMapperFactory(
	edgeMapFactory func(aspect_ratio float64) func(sobelProv SobelProvider, x, y int) rune,
) asciioption {
	return func(a *AsciiConverter) {
		a.EdgeMapperFactory = edgeMapFactory
	}
}

func WithDefaultEdgeMapperFactory() asciioption {
	return WithEdgeMapperFactory(defaultEdgeMapperFactory)
}

func WithColorMapper(
	colorMapper func(lumProv LuminosityProvider, x int, y int) (int, string),
) asciioption {
	return func(a *AsciiConverter) {
		a.ANSIColorMapper = colorMapper
	}
}

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
	DoBlackThreshold: true,
	BlackLumUpper: 50,
	DoWhiteThreshold: true,
	WhiteLumLower: 200,
}

func Default3BitColorMapper() func(LuminosityProvider, int, int) (int, string) {
	return default3BitColorMapperFactory(default3BitOpts)
}

func Default4BitColorMapper() func(LuminosityProvider, int, int) (int, string) {
	opts := ColorMapper4BitOptions {
		ColorMapper3BitOptions: default3BitOpts,
		BoldColoredLumLower: 100,
		BoldBlackLumLower: 40,
		BoldWhiteLumLower: 240,
	}

	return default4BitColorMapperFactory(opts)
}

func Default8BitColorMapper() func(LuminosityProvider, int, int) (int, string) {
	opts := ColorMapper8BitOptions {
		rStep: [3]int{0, 95, 40},
		gStep: [3]int{0, 95, 40},
		bStep: [3]int{0, 95, 40},
		greyStep: [3]int{8, 18, 10},
	}

	return default8BitColorMapperFactory(opts)
}

func Default24BitColorMapper() func(LuminosityProvider, int, int) (int, string) {
	return default24BitColorMapperFactory()
}

func WithDefaultColorMapper() asciioption {
	return WithColorMapper(defaultColorMapper())
}

func WithDefault3BitColorMapper() asciioption {
	return func(a *AsciiConverter) {
		a.ANSIColorMapper = Default3BitColorMapper()
		a.BytesPerCharToReserve = bytesPerCharReserve
		a.AdditionalBytesPerCharColor = ansiAdditionalBytesReserved3Bit
	}
}

func WithDefault4BitColorMapper() asciioption {
	return func(a *AsciiConverter) {
		a.ANSIColorMapper = Default4BitColorMapper()
		a.BytesPerCharToReserve = bytesPerCharReserve
		a.AdditionalBytesPerCharColor = ansiAdditionalBytesReserved4Bit
	}
}

func WithDefault8BitColorMapper() asciioption {
	return func(a *AsciiConverter) {
		a.ANSIColorMapper = Default8BitColorMapper()
		a.BytesPerCharToReserve = bytesPerCharReserve
		a.AdditionalBytesPerCharColor = ansiAdditionalBytesReserved8Bit
	}
}

func WithDefault24BitColorMapper() asciioption {
	return func(a *AsciiConverter) {
		a.ANSIColorMapper = Default24BitColorMapper()
		a.BytesPerCharToReserve = bytesPerCharReserve
		a.AdditionalBytesPerCharColor = ansiAdditionalBytesReserved24Bit
	}
}

// func defaultLuminenceMapperFactory() func(luminosityProvider, int) rune {
	// return defaultLuminenceMapper
// }

func defaultLuminenceMapper(lumProv LuminosityProvider, x, y int) rune {
	const charRamp = `$@B%8&WM#*oahkbdpqwmZO0QLCJUYXzcvunxrjft/\|()1{}[]?-_+~<>i!lI;:,"^` + "`" + `'. `
	const charLen = float64(len(charRamp))

	luminence := lumProv.LuminosityAt(x, y)
	charIdx := len(charRamp) - int(float64(luminence) / 255 * (charLen - 1)) - 1

	return rune(charRamp[charIdx])
}

func defaultEdgeMapperFactory(aspect_ratio float64) func(SobelProvider, int, int) rune {
	type edgeGlyphStop struct {
		rune
		float64
	}

	const minGrad = float64(-60.5)
	const maxGrad = float64(60.5)

	stops := [...]edgeGlyphStop{
		{rune: '-', float64: math.Nextafter(-60, math.Inf(-1))},		// (-inf, -7)
		{rune: 'l', float64: math.Nextafter(-40, math.Inf(-1))},		// [-7, -5)
		{rune: 'L', float64: math.Nextafter(-30, math.Inf(-1))},		// [-5, -3)
		{rune: '\\', float64: math.Nextafter(-3, math.Inf(-1))},	// [-3, -0.5)
		{rune: '|', float64: 3},									// [-0.5, 0.5]
		{rune: '/', float64: math.Nextafter(30, math.Inf(1))},		// (0.5, 3]
		{rune: 'J', float64: math.Nextafter(40, math.Inf(1))},		// (3, 5]
		{rune: 'j', float64: math.Nextafter(60, math.Inf(1))},		// (5, 7]
		{rune: '-', float64: math.Inf(1)},							// (7, inf)
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

func With3BitColorMapper(opts ColorMapper3BitOptions, bytesPerCharToReserve, colorBytesPerCharToReserve float64) asciioption {
	return func(a *AsciiConverter) {
		a.BytesPerCharToReserve = bytesPerCharToReserve
		a.AdditionalBytesPerCharColor = colorBytesPerCharToReserve

		a.ANSIColorMapper = default3BitColorMapperFactory(opts)
	}
}

func With4BitColorMapper(opts ColorMapper4BitOptions, bytesPerCharToReserve, colorBytesPerCharToReserve float64) asciioption {
	return func(a *AsciiConverter) {
		a.BytesPerCharToReserve = bytesPerCharToReserve
		a.AdditionalBytesPerCharColor = colorBytesPerCharToReserve
		a.ANSIColorMapper = default4BitColorMapperFactory(opts)
	}
}

func With8BitColorMapper(opts ColorMapper8BitOptions, bytesPerCharToReserve, colorBytesPerCharToReserve float64) asciioption {
	return func(a *AsciiConverter) {
		a.BytesPerCharToReserve = bytesPerCharToReserve
		a.AdditionalBytesPerCharColor = colorBytesPerCharToReserve
		a.ANSIColorMapper = default8BitColorMapperFactory(opts)
	}
}

func With24BitColorMapper(opts ColorMapper24BitOptions, bytesPerCharToReserve, colorBytesPerCharToReserve float64) asciioption {
	return func(a *AsciiConverter) {
		a.BytesPerCharToReserve = bytesPerCharToReserve
		a.AdditionalBytesPerCharColor = colorBytesPerCharToReserve
		a.ANSIColorMapper = default24BitColorMapperFactory()
	}
}

func (a *AsciiConverter) ConvertReader(r io.Reader, targetWidth, targetHeight int) (string, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return "", err
	}

	return a.Convert(img, targetWidth, targetHeight), nil
}

func (a *AsciiConverter) ConvertBytes(b []byte, targetWidth, targetHeight int) (string, error) {
	return a.ConvertReader(bytes.NewReader(b), targetWidth, targetHeight)
}

/*
DownscaleImage downscales the src image using the DownscaleFactor field of the asciiconverter struct. This function is intended to be used to downscale before any processing happens. It will never downscale below the targetWidth or targetHeight, and will scale the shorter measure in accordance to the aspect ratio. For example, if the AspectRatio=2 (the most common scenario for terminals, because width of output is twice the height to account for 1:2 character dimensions), then the height will be shrunk down by a factor of 1/2. Conversely, if the AspectRatio=0.5 (for the case when a character is twice as long as it is tall, so output image is twice as short to compensate), then the width will be shrunk down by a factor of 1/2.

Alternatively, if you want to downscale directly to the targetWidth/targetHeight, set IgnoreAspectRatio = true.
That will signal the function to always downscale to the target resolution. In the common case of AspectRatio=2 (>1), it will ignore the targetHeight and forcibly downscale the height (disregarding targetHeight) so that it is in the correct aspectRatio

If DownscaleFactor < 1, then the function will set the value to 1 (<0 values not allowed, no upscaling)

Returns the downscaled image, and the effective aspect ratio. The effective aspect ratio is approximately equal to the original aspect ratio, but may differ because of integer clamping. Use the effective aspect ratio to adjust Sobel thresholds or gradient correction, since the sampling grid may differ slightly from OutputAspectRatio due to integer rounding.
*/
func (a *AsciiConverter) DownscaleImage(src image.Image, targetWidth, targetHeight int) (image.Image, float64) {
	if a.DownscaleFactor < 1 {
		a.DownscaleFactor = 1
	}

	// Check if we need to do anything
	if a.DownscaleFactor == 1 && a.OutputAspectRatio == 1 {
		return src, a.OutputAspectRatio
	}

	var newWidth, newHeight int
	srcBounds := src.Bounds()
	srcWidth, srcHeight := srcBounds.Dx(), srcBounds.Dy()

	// NOTE: We will never upscale width or height. Instead, downscale the opposing axis.
	if a.AlwaysDownscaleToTarget {
		if a.OutputAspectRatio >= 1 {
			// aspect ratio >= 1, so downscale directly to the width, then scale height accordingly
			newWidth = targetWidth
			newHeight = int(float64(targetWidth) / a.OutputAspectRatio)
		} else {
			// aspect ratio < 1, so downscale directly to the height, then scale width accordingly
			newWidth = int(float64(targetHeight) * a.OutputAspectRatio)
			newHeight = targetHeight
		}
		
	} else {
		if a.OutputAspectRatio >= 1 {
			// scale width down by scale factor
			newWidth = max(int(float64(srcWidth) / a.DownscaleFactor), targetWidth)
			// scale height accordingly with respect to the width and aspect ratio
			newHeight = int(float64(newWidth) / a.OutputAspectRatio)
		} else {
			// scale height down by scale factor
			newHeight = max(int(float64(srcHeight) / a.DownscaleFactor), targetHeight)
			// scale width accordingly with respect to the height and aspect ratio
			newWidth = int(float64(newHeight) * a.OutputAspectRatio)
		}
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
			// TODO: Add other downsampling methods, currently using nearest neighbour

			srcX := int(float64(x) * float64(srcWidth) / float64(newWidth))
			srcY := int(float64(y) * float64(srcHeight) / float64(newHeight))

			c := src.At(srcX, srcY)

			downscaledImg.Set(x, y, c)
		}
	}

	return downscaledImg, float64(newWidth) / float64(newHeight)
}

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

func applySobelCentralPixel(lumImg LuminosityProvider, gGrad []float64, gMag2 []int, gLap []int, x, y int) {
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

	l := +1 * lumImg.LuminosityAt(x, y-1) +
		+1 * lumImg.LuminosityAt(x-1,y) + 
		-4 * lumImg.LuminosityAt(x,y) +
		+1 * lumImg.LuminosityAt(x+1,y) +
		+1 * lumImg.LuminosityAt(x,y+1)

	gLap[idx] = l
}

func applySobelPixelSafely(lumImg LuminosityProvider, gGrad []float64, gMag2 []int, gLap []int, x, y int) {
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

	l := +1 * lumImg.SafeLuminosityAt(x, y-1) +
		+1 * lumImg.SafeLuminosityAt(x-1,y) + 
		-4 * lumImg.SafeLuminosityAt(x,y) +
		+1 * lumImg.SafeLuminosityAt(x+1,y) +
		+1 * lumImg.SafeLuminosityAt(x,y+1)

	gLap[idx] = l
}

func (a *AsciiConverter) ApplySobel(lumImg LuminosityProvider) defaultSobelProvider {	
	gWidth := lumImg.Width()
	gHeight := lumImg.Height()

	gLen := gWidth * gHeight
	gMag2 := make([]int, gLen)
	gGrad := make([]float64, gLen)
	gLap := make([]int, gLen)

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

func (a *AsciiConverter) ASCIIGenWithSobel(sobelProv SobelProvider, aspect_ratio float64) string {
	adjustedGMag2Threshold := int(a.SobelMagnitudeThreshold * (aspect_ratio * aspect_ratio))

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

			if sobelProv.SobelMag2At(x, y) >= adjustedGMag2Threshold {
				// if code != prevColor {
					// prevColor = (0 >> 16) | (0 >> 8) | 255
//
					// asciiBuilder.WriteString("\x1b[38;2;0;0;255m")
				// }

				if !prevWasBold {
					prevWasBold = true
					asciiBuilder.WriteString("\x1b[1m")
				}

				asciiBuilder.WriteRune(edgeMapper(sobelProv, x, y))
			} else {
				// if code != prevColor {
					// prevColor = code
//
					// asciiBuilder.WriteString(escapeStr)
				// }

				if prevWasBold {
					prevWasBold = false
					asciiBuilder.WriteString("\x1b[22m")
				}

				asciiBuilder.WriteRune(a.LuminenceMapper(sobelProv, x, y))
			}
		}
		asciiBuilder.WriteRune('\n')
	}

	asciiBuilder.WriteString("\x1b[0m")
	
	return asciiBuilder.String()
}

func (a *AsciiConverter) ASCIIGen(lumProv LuminosityProvider, aspect_ratio float64) string {
	width, height := lumProv.Width(), lumProv.Height()
	// numPixels := width * height

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

				asciiBuilder.WriteRune(a.LuminenceMapper(lumProv, x, y))
			}
			asciiBuilder.WriteRune('\n')
		}

		asciiBuilder.WriteString("\x1b[0m")

	} else {

		for y := range height {
			for x := range width {
				asciiBuilder.WriteRune(a.LuminenceMapper(lumProv, x, y))
			}
			asciiBuilder.WriteRune('\n')
		}

		asciiBuilder.WriteString("\x1b[0m")
	}
	
	return asciiBuilder.String()
}

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
