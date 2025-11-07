package asciiart

import (
	"bytes"
	"image"
	"image/color"
	"io"
	"math"
	"strings"
)

const (
	bytesPerPixel						= 3.5 // assume avg 3.5 bytes per pixel
	ansiBytesPerPixel 					= 2 // reserve an extra 2 bytes per pixel to allow room for ANSI escape sequences
)

// const (
	// ansi_reset			ANSIEnumVal	= iota
	// ansi_fg_black
	// ansi_fg_green
	// ansi_fg_yellow
	// ansi_fg_blue
	// ansi_fg_magenta
	// ansi_fg_cyan
	// ansi_fg_white
	// ansi_fg_bright_black
	// ansi_fg_bright_red
	// ansi_fg_bright_green
	// ansi_fg_bright_yellow
	// ansi_fg
// )
//
// var (
	// ansi_esc_codes = [...]string{
		// "\x1b[0m",
		// "\x1b[31m",
		// "\x1b[32m",
	// }
// )
//
// type ANSIEnumVal int
//
// type ANSIEnum struct { }
//
// func (a ANSIEnum) ANSI_RESET() ANSIEnumVal { return ansi_reset }

type ColorMapper3BitOptions struct {
	BlackLumUpperThreshold int
	WhiteLumLowerThreshold int
	ColorMapperOptions
}

type ColorMapper4BitOptions struct {
	ColorMapperOptions
}

type ColorMapper8BitOptions struct{
	ColorMapperOptions
}

type ColorMapperOptions struct {
	ColorAdd	[3]int
	ColorScale	[3]float64
}

type asciiconverter struct {
	/* 
	DownscaleFactor is the scale factor at which the image is downsampled using nearest neighbour sampling
	without going below targetWidth and targetHeight when calling Convert().

	Any downscale factor < 1 will be interpreted (and potentially updated) to 1. This is because upscaling is not allowed.
	*/
	DownscaleFactor				float64
	// EdgeThreshold provides the gMag2 value threshold before an edge is registered as an edge. This field only has an effect if UseSobel is true.
	EdgeThreshold				float64
	// OutputAspectRatio is the aspect ratio of the resulting image (char_x / char_y).
	// In most cases, a terminal character's height is twice its width. 
	// So the resulting image must be 2:1 ratio to compensate for the taller height
	OutputAspectRatio 			float64
	// IgnoreAspectRatio will ignore aspect ratio and forceably downscale the targetWidth, targetHeight. It is recommended to set this to false, and just configure the AspectRatio normally (default=2, and in most cases, it will work fine)
	IgnoreAspectRatio			bool
	// UseSobel flags to the converter whether or not sobel edge detection should be used.
	UseSobel					bool
	// UseColor flags to the converter whether terminal escape sequences used to indicate colour should be used
	UseColor					bool
	// The function that converts a luminence value (0-255) to a rune
	LuminenceMapperFactory		func(aspect_ratio float64) func(lumProv luminosityProvider, idx int) rune
	// The function that converts an approximate gradient to a rune
	EdgeMapperFactory			func(aspect_ratio float64) func(sobelProv sobelProvider, idx int) rune
	ANSIColorMapper				func(color.Color, int) string
}

type asciioption func(*asciiconverter)

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
	luminosityProvider
	G_Grad		[]float64
	G_Mag2		[]int
}

func makeDefaultSobelProvider(lumProvider luminosityProvider, gGrad []float64, gMag2 []int) defaultSobelProvider {
	return defaultSobelProvider{
		luminosityProvider: lumProvider,
		G_Grad: gGrad,
		G_Mag2: gMag2,
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

/*
SobelMag2At returns the sobel magnitude squared at some x, y pixel. However, if you need 1D iteration, use x as the iterating variable, and set y = 0. The SobelMag2At() function does not check if x and y are actually valid. Essentially under the hood it is doing:

	return d.gMag2[x + y * d.width]
*/
func (d defaultSobelProvider) SobelMag2At(x, y int) int {
	return d.G_Mag2[x + y * d.Width()]
}

type luminosityProvider interface {
	image.Image
	LuminosityAt1D(int) int
	LuminosityAt(int, int) int
	SafeLuminosityAt(int, int) int
	LuminositySet1D(int, int)
	LuminositySet(int, int, int)
	Width() int
	Height() int
}

type sobelProvider interface {
	image.Image
	luminosityProvider
	SobelEdgeDetected(int, int, int) bool
	SobelGradAt1D(int) float64
	SobelGradAt(int, int) float64
	SobelMag2At1D(int) int
	SobelMag2At(int, int) int
}

/*
NewDefault initialises an asciiart instance with default parameters.

	- DownscaleFactor: 1
	- EdgeStrength: 1
	- AspectRatio: 2
	- UseSobel: true
	- DefaultLuminenceToCharMapping

*/
func NewDefault() *asciiconverter {
	return &asciiconverter {
		DownscaleFactor: 0,
		EdgeThreshold: 1,
		OutputAspectRatio: 2,
		IgnoreAspectRatio: false,
		UseColor: true,
		UseSobel: true,
		LuminenceMapperFactory: defaultLuminenceMapperFactory,
		EdgeMapperFactory: defaultEdgeMapperFactory,
	}
}

func New(opts ...asciioption) *asciiconverter {
	ascii := NewDefault()

	for _, o := range opts {
		o(ascii)
	}

	return ascii
}

func WithDownscaleFactor(factor float64) asciioption {
	return func(a *asciiconverter) {
		a.DownscaleFactor = factor
	}
}

func WithEdgeStrength(strength float64) asciioption {
	return func(a *asciiconverter) {
		a.EdgeThreshold = strength
	}
}

func WithAspectRatio(ratio float64) asciioption {
	return func(a *asciiconverter) {
		a.OutputAspectRatio = ratio
	}
}

func WithIgnoreAspectRatio(ignore bool) asciioption {
	return func(a *asciiconverter) {
		a.IgnoreAspectRatio = ignore
	}
}

func WithSobel(useSobel bool) asciioption {
	return func(a *asciiconverter) {
		a.UseSobel = useSobel
	}
}

func WithColor(useColor bool) asciioption {
	return func(a *asciiconverter) {
		a.UseColor = useColor
	}
}

func WithLuminenceMapperFactory(
	lumMapFactory func(aspect_ratio float64) func(lumProv luminosityProvider, idx int) rune,
) asciioption {
	return func(a *asciiconverter) {
		a.LuminenceMapperFactory = lumMapFactory
	}
}

func WithEdgeMapperFactory(
	edgeMapFactory func(aspect_ratio float64) func(sobelProv sobelProvider, idx int) rune,
) asciioption {
	return func(a *asciiconverter) {
		a.EdgeMapperFactory = edgeMapFactory
	}
}

func WithColorMapper(
	colorMapper func(color.Color, int) string,
) asciioption {
	return func(a *asciiconverter) {
		a.ANSIColorMapper = colorMapper
	}
}

func defaultLuminenceMapperFactory(aspect_ratio float64) func(luminosityProvider, int) rune {
	return func(lumProv luminosityProvider, idx int) rune {
		
		return ' '
	}
}

func defaultEdgeMapperFactory(aspect_ratio float64) func(sobelProvider, int) rune {
	return func(sobelProv sobelProvider, idx int) rune {

		return ' '
	}
}

func With3BitColorMapper(opts ColorMapper3BitOptions) asciioption {
	return func(a *asciiconverter) {
		a.ANSIColorMapper = default3BitColorMapperFactory(opts)
	}
}

func With4BitColorMapper(opts ColorMapper4BitOptions) asciioption {
	return func(a *asciiconverter) {
		a.ANSIColorMapper = default4BitColorMapperFactory(opts)
	}
}

func With8BitColorMapper(opts ColorMapper8BitOptions) asciioption {
	return func(a *asciiconverter) {
		a.ANSIColorMapper = default8BitColorMapperFactory(opts)
	}
}

func (a *asciiconverter) ConvertReader(r io.Reader, targetWidth, targetHeight int) (string, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return "", err
	}

	return a.Convert(img, targetWidth, targetHeight), nil
}

func (a *asciiconverter) ConvertBytes(b []byte, targetWidth, targetHeight int) (string, error) {
	return a.ConvertReader(bytes.NewReader(b), targetWidth, targetHeight)
}

/*
DownscaleImage downscales the src image using the DownscaleFactor field of the asciiconverter struct. This function is intended to be used to downscale before any processing happens. It will never downscale below the targetWidth or targetHeight, and will scale the shorter measure in accordance to the aspect ratio. For example, if the AspectRatio=2 (the most common scenario for terminals, because width of output is twice the height to account for 1:2 character dimensions), then the height will be shrunk down by a factor of 1/2. Conversely, if the AspectRatio=0.5 (for the case when a character is twice as long as it is tall, so output image is twice as short to compensate), then the width will be shrunk down by a factor of 1/2.

Alternatively, if you want to downscale directly to the targetWidth/targetHeight, set IgnoreAspectRatio = true.
That will signal the function to always downscale to the target resolution. In the common case of AspectRatio=2 (>1), it will ignore the targetHeight and forcibly downscale the height (disregarding targetHeight) so that it is in the correct aspectRatio

If DownscaleFactor < 1, then the function will set the value to 1 (<0 values not allowed, no upscaling)

Returns the downscaled image, and the effective aspect ratio. The effective aspect ratio is approximately equal to the original aspect ratio, but may differ because of integer clamping. Use the effective aspect ratio to adjust Sobel thresholds or gradient correction, since the sampling grid may differ slightly from OutputAspectRatio due to integer rounding.
*/
func (a *asciiconverter) DownscaleImage(src image.Image, targetWidth, targetHeight int) (image.Image, float64) {
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

	// DownscaleFactor < 1 indicates that the src image be forcibly resized to the targetWidth
	// NOTE: We will never upscale width or height. Instead, downscale the opposing axis.
	if a.IgnoreAspectRatio {
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

func (a *asciiconverter) MapLuminosity(img image.Image) defaultLuminosityProvider {
	lumImg := makeDefaultLuminosityImage(img)
	
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	for x := range width {
		for y := range height {
			// Ignore transparency
			r, g, b, _ := img.At(x, y).RGBA()
			lum := int(0.2126 * float64(r) + 0.7152 * float64(g) + 0.0722 * float64(b))
			lumImg.LuminositySet(x, y, lum)
		}
	}

	return lumImg
}

func computeGrad(x float64, y float64) float64 {
	if x == 0 {
		return math.MaxFloat64
	} else {
		return x / y
	}
}

func applySobelCentralPixel(lumImg luminosityProvider, gGrad []float64, gMag2 []int, x, y int) {
	cur_gx := -1 * lumImg.LuminosityAt(x-1,y-1) +
	+1 * lumImg.LuminosityAt(x+1,y-1) +
	-2 * lumImg.LuminosityAt(x-1,y) +
	+2 * lumImg.LuminosityAt(x+1,y) +
	-1 * lumImg.LuminosityAt(x-1,y+1) +
	+1 * lumImg.LuminosityAt(x+1,y+1)

	cur_gy := -1 * lumImg.LuminosityAt(x-1,y-1) +
	-2 * lumImg.LuminosityAt(x,y-1) +
	-1 * lumImg.LuminosityAt(x+1,y-1) +
	+1 * lumImg.LuminosityAt(x-1,y+1) +
	+2 * lumImg.LuminosityAt(x,y+1) +
	+1 * lumImg.LuminosityAt(x+1,y+1)

	// Normally, we would have to scale the gMag2 to account for the aspect ratio.
	cur_gMag2 := cur_gx * cur_gx + cur_gy * cur_gy
	idx := x + y * lumImg.Width()

	gMag2[idx] = cur_gMag2
	// This gradient is not normalised. Normally you would multiply by dX / dY to account for it.
	// Instead during lum->char translations, we will multiply the grad thresholds by dY/dX to be more efficient
	gGrad[idx] = computeGrad(float64(cur_gx), float64(cur_gy))
}

func applySobelPixelSafely(lumImg luminosityProvider, gGrad []float64, gMag2 []int, x, y int) {
	cur_gx := -1 * lumImg.SafeLuminosityAt(x-1,y-1) +
	+1 * lumImg.SafeLuminosityAt(x+1,y-1) +
	-2 * lumImg.SafeLuminosityAt(x-1,y) +
	+2 * lumImg.SafeLuminosityAt(x+1,y) +
	-1 * lumImg.SafeLuminosityAt(x-1,y+1) +
	+1 * lumImg.SafeLuminosityAt(x+1,y+1)

	cur_gy := -1 * lumImg.SafeLuminosityAt(x-1,y-1) +
	-2 * lumImg.SafeLuminosityAt(x,y-1) +
	-1 * lumImg.SafeLuminosityAt(x+1,y-1) +
	+1 * lumImg.SafeLuminosityAt(x-1,y+1) +
	+2 * lumImg.SafeLuminosityAt(x,y+1) +
	+1 * lumImg.SafeLuminosityAt(x+1,y+1)

	// Normally, we would have to scale the gMag2 to account for the aspect ratio.
	cur_gMag2 := cur_gx * cur_gx + cur_gy * cur_gy
	idx := x + y * lumImg.Width()

	gMag2[idx] = cur_gMag2
	// This gradient is not normalised. Normally you would multiply by dX / dY to account for it.
	// Instead during lum->char translations, we will multiply the grad thresholds by dY/dX to be more efficient
	gGrad[idx] = computeGrad(float64(cur_gx), float64(cur_gy))
}

func (a *asciiconverter) ApplySobel(lumImg luminosityProvider) defaultSobelProvider {	
	gWidth := lumImg.Width()
	gHeight := lumImg.Height()

	gLen := gWidth * gHeight
	gMag2 := make([]int, gLen)
	gGrad := make([]float64, gLen)

	// Calculate G
	for y := 1; y < gHeight - 1; y++ {
		for x := 1; x < gWidth - 1; x++ {
			applySobelCentralPixel(lumImg, gGrad, gMag2, x, y)
		}
	}

	// Apply left/right sides
	for x := range gWidth {
		applySobelPixelSafely(lumImg, gGrad, gMag2, x, 0)
		applySobelPixelSafely(lumImg, gGrad, gMag2, x, gHeight - 1)
	}

	// Apply bottom/top (skipping corners that we have already done)
	for y := 1; y < gHeight - 1; y++ {
		applySobelPixelSafely(lumImg, gGrad, gMag2, 0, y)
		applySobelPixelSafely(lumImg, gGrad, gMag2, gWidth-1, y)
	}

	return makeDefaultSobelProvider(lumImg, gGrad, gMag2)
}

func (a *asciiconverter) ASCIIGenWithSobel(sobelProv sobelProvider, aspect_ratio float64) string {
	adjustedGMag2Threshold := a.EdgeThreshold * (aspect_ratio * aspect_ratio)

	width, height := sobelProv.Width(), sobelProv.Height()
	numPixels := width * height

	lumMapper := a.LuminenceMapperFactory(aspect_ratio)
	edgeMapper := a.EdgeMapperFactory(aspect_ratio)

	var bufferSize int
	if a.UseColor {
		// In most cases, we will overallocate by a few hundred bytes to ensure there is no reallocation of the buffer
		// This is because it cannot be known how much room should be left for the colour ANSI escape sequences
		bufferSize = int((bytesPerPixel + ansiBytesPerPixel) * float64(width + 1) * float64(height)) // width + 1 because leave a byte for the new line byte
	} else {
		bufferSize = int(bytesPerPixel * float64(width + 1) * float64(height)) // width + 1 because leave a byte for the new line
	}

	var asciiBuilder strings.Builder
	asciiBuilder.Grow(bufferSize)
	for i := range numPixels {
		currMag2 := sobelProv.SobelMag2At1D(i)
		if currMag2 >= int(adjustedGMag2Threshold) {
			asciiBuilder.WriteRune(lumMapper(sobelProv, i))
		} else {
			asciiBuilder.WriteRune(edgeMapper(sobelProv, i))
		}
	}
	
	return asciiBuilder.String()
}

func (a *asciiconverter) ASCIIGen(lumProv luminosityProvider, aspect_ratio float64) string {
	width, height := lumProv.Width(), lumProv.Height()
	numPixels := width * height

	lumMapper := a.LuminenceMapperFactory(aspect_ratio)

	var bufferSize int
	if a.UseColor {
		// In most cases, we will overallocate by a few hundred bytes to ensure there is no reallocation of the buffer
		// This is because it cannot be known how much room should be left for the colour ANSI escape sequences
		bufferSize = int((bytesPerPixel + ansiBytesPerPixel) * float64(width + 1) * float64(height)) // width + 1 because leave a byte for the new line byte
	} else {
		bufferSize = int(bytesPerPixel * float64(width + 1) * float64(height)) // width + 1 because leave a byte for the new line
	}

	var asciiBuilder strings.Builder
	asciiBuilder.Grow(bufferSize)
	for i := range numPixels {
		asciiBuilder.WriteRune(lumMapper(lumProv, i))
	}
	
	return asciiBuilder.String()
}

func (a *asciiconverter) Convert(img image.Image, targetWidth, targetHeight int) string {
	var effectiveAspectRatio float64
	img, effectiveAspectRatio = a.DownscaleImage(img, targetWidth, targetHeight)
	lumImg := a.MapLuminosity(img)

	if a.UseSobel {
		sobelImg := a.ApplySobel(lumImg)

		return a.ASCIIGenWithSobel(sobelImg, effectiveAspectRatio)
	}

	return a.ASCIIGen(lumImg, effectiveAspectRatio)
}

