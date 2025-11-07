package asciiart

import "image/color"

func default3BitColorMapperFactory(
	opts ColorMapper3BitOptions,
) func(color.Color, int) string {
	return func(c color.Color, lum int) string {
		return ""
	}
}

func default4BitColorMapperFactory(opts ColorMapper4BitOptions) func(color.Color, int) string {
	return func(c color.Color, lum int) string {
		return ""
	}
}

func default8BitColorMapperFactory(opts ColorMapper8BitOptions) func(color.Color, int) string {
	return func(c color.Color, lum int) string {
		return ""
	}
}
