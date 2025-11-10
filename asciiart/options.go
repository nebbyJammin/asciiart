package asciiart

// WithEdgeStrength specifies a minimum Sobel Magnitude Squared for a char to be registered as an edge. It is highly recommended to go with a value between 10,000-120,000
func WithEdgeStrength(strength float64) asciioption {
	return func(a *AsciiConverter) {
		a.SobelMagnitudeThresholdNormalized = strength
	}
}

// WithAspectRatio specifies desired aspect_ratio of the image. This field is only used if DownscalingMode is set to DownscalingModes.WithRespectToAspectRatio()
func WithAspectRatio(ratio float64) asciioption {
	return func(a *AsciiConverter) {
		a.OutputAspectRatio = ratio
	}
}

/*
WithDownscalingMode specifies how the ascii converter should downscale the image. It is recommended to use the default DownscalingModes.WithRespectToAspectRatio().

TODO: Docs for downscaling mode
*/
func WithDownscalingMode(mode DownscalingMode) asciioption {
	return func(a *AsciiConverter) {
		a.DownscalingMode = mode
	}
}

/*
WithSobel enables/disables sobel edge detection. In general, use sobel edge detection only when the target size is big enough (approx >=100x100). Generally, with low resolution, sobel edge detection cannot reliably detect edges without looking noisy.
*/
func WithSobel(useSobel bool) asciioption {
	return func(a *AsciiConverter) {
		a.UseSobel = useSobel
	}
}

/*
WithColor enables/disables color. Use this in combination with the color mapper. 

NOTE: Ensure that your terminal supports the color space of each color mapper. This library implements some default mappers for most standard ANSI color escape sequences (3bit, 4bit, 8bit and 24bit color space).
*/
func WithColor(useColor bool) asciioption {
	return func(a *AsciiConverter) {
		a.UseColor = useColor
	}
}

/*
WithLuminosityMapper specifies a luminosity mapper to use. A luminosity mapper maps a luminosity value (0-255) onto some character. It does not interpret the color (see WithColorMapper), it only provides the character that should be used for a normal character.
*/
func WithLuminosityMapper(
	lumMapper func(lumProv LuminosityProvider, x, y int) rune,
) asciioption {
	return func(a *AsciiConverter) {
		a.LuminenceMapper = lumMapper
	}
}

/*
WithDefaultLuminenceMapper uses the default luminosity mapper provided by this library
*/
func WithDefaultLuminenceMapper() asciioption {
	return WithLuminosityMapper(DefaultLuminenceMapper)
}

/*
WithEdgeMapperFactory specifies an edge mapper factory to use. As opposed to the luminosity mapper, this needs to be a factory, because edge gradients need to be adjusted depending on the target aspect ratio. This is due to the fact that different aspect ratios will have a different effect on the resulting sobel gradient and magnitude
*/
func WithEdgeMapperFactory(
	edgeMapFactory func(aspect_ratio float64) func(sobelProv SobelProvider, x, y int) rune,
) asciioption {
	return func(a *AsciiConverter) {
		a.EdgeMapperFactory = edgeMapFactory
	}
}

/*
WithDefaultEdgeMapperFactory uses the default edge mapper provided by this library
*/
func WithDefaultEdgeMapperFactory() asciioption {
	return WithEdgeMapperFactory(DefaultEdgeMapperFactory)
}

/*
WithColorMapper specifies a color mapper to use. The color mapper takes in a LuminosityProvider and an x, y character position and returns
	- The character code (OR a unique identifier)
	- The formatted ANSI escape sequence to be inserted into the final result string
*/
func WithColorMapper(
	colorMapper func(lumProv LuminosityProvider, x int, y int) (int, string),
) asciioption {
	return func(a *AsciiConverter) {
		a.ANSIColorMapper = colorMapper
	}
}

/*
WithDefaultColorMapper sets the ascii converter to use the default color map. The default color mapper is the 3 Bit Color Mapper implemented by this library.
*/
func WithDefaultColorMapper() asciioption {
	return WithColorMapper(defaultColorMapper())
}

/*
WithDefault3BitColorMapper sets the ascii converter to use the default configuration for the library implementation of 3 bit color map.
*/
func WithDefault3BitColorMapper() asciioption {
	return func(a *AsciiConverter) {
		a.ANSIColorMapper = Default3BitColorMapper()
		a.BytesPerCharToReserve = bytesPerCharReserve
		a.AdditionalBytesPerCharColor = ansiAdditionalBytesReserved3Bit
	}
}

/*
WithDefault4BitColorMapper sets the ascii converter to use the default configuration for the library implementation of 4 bit color map.
*/
func WithDefault4BitColorMapper() asciioption {
	return func(a *AsciiConverter) {
		a.ANSIColorMapper = Default4BitColorMapper()
		a.BytesPerCharToReserve = bytesPerCharReserve
		a.AdditionalBytesPerCharColor = ansiAdditionalBytesReserved4Bit
	}
}

/*
WithDefault8BitColorMapper sets the ascii converter to use the default configuration for the library implementation of 8 bit color map.
*/
func WithDefault8BitColorMapper() asciioption {
	return func(a *AsciiConverter) {
		a.ANSIColorMapper = Default8BitColorMapper()
		a.BytesPerCharToReserve = bytesPerCharReserve
		a.AdditionalBytesPerCharColor = ansiAdditionalBytesReserved8Bit
	}
}

/*
WithDefault24BitColorMapper sets the ascii converter to use the default configuration for the library implementation of 24 bit color map.
*/
func WithDefault24BitColorMapper() asciioption {
	return func(a *AsciiConverter) {
		a.ANSIColorMapper = Default24BitColorMapper()
		a.BytesPerCharToReserve = bytesPerCharReserve
		a.AdditionalBytesPerCharColor = ansiAdditionalBytesReserved24Bit
	}
}

/*
With3BitColorMapper signals to the ascii converter to use the default 3 bit color mapper with opts as the configuration. Specify the bytesPerCharToReserve and colorBytesPerCharToReserve. If you do not plan on using color, just use 0 for colorBytesPerCharToReserve.
*/
func With3BitColorMapper(opts ColorMapper3BitOptions, bytesPerCharToReserve, colorBytesPerCharToReserve float64) asciioption {
	return func(a *AsciiConverter) {
		a.BytesPerCharToReserve = bytesPerCharToReserve
		a.AdditionalBytesPerCharColor = colorBytesPerCharToReserve

		a.ANSIColorMapper = default3BitColorMapperFactory(opts)
	}
}

/*
With4BitColorMapper signals to the ascii converter to use the default 4 bit color mapper with opts as the configuration. Specify the bytesPerCharToReserve and colorBytesPerCharToReserve. If you do not plan on using color, just use 0 for colorBytesPerCharToReserve.
*/
func With4BitColorMapper(opts ColorMapper4BitOptions, bytesPerCharToReserve, colorBytesPerCharToReserve float64) asciioption {
	return func(a *AsciiConverter) {
		a.BytesPerCharToReserve = bytesPerCharToReserve
		a.AdditionalBytesPerCharColor = colorBytesPerCharToReserve
		a.ANSIColorMapper = default4BitColorMapperFactory(opts)
	}
}

/*
With8BitColorMapper signals to the ascii converter to use the default 8 bit color mapper with opts as the configuration. Specify the bytesPerCharToReserve and colorBytesPerCharToReserve. If you do not plan on using color, just use 0 for colorBytesPerCharToReserve.
*/
func With8BitColorMapper(opts ColorMapper8BitOptions, bytesPerCharToReserve, colorBytesPerCharToReserve float64) asciioption {
	return func(a *AsciiConverter) {
		a.BytesPerCharToReserve = bytesPerCharToReserve
		a.AdditionalBytesPerCharColor = colorBytesPerCharToReserve
		a.ANSIColorMapper = default8BitColorMapperFactory(opts)
	}
}

/*
With24BitColorMapper signals to the ascii converter to use the default 24 bit color mapper. Specify the bytesPerCharToReserve and colorBytesPerCharToReserve. If you do not plan on using color, just use 0 for colorBytesPerCharToReserve.
*/
func With24BitColorMapper(bytesPerCharToReserve, colorBytesPerCharToReserve float64) asciioption {
	return func(a *AsciiConverter) {
		a.BytesPerCharToReserve = bytesPerCharToReserve
		a.AdditionalBytesPerCharColor = colorBytesPerCharToReserve
		a.ANSIColorMapper = default24BitColorMapperFactory()
	}
}
