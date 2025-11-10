package asciiart

import (
	"fmt"
	"image/color"
)

func giveRewards(awards []int, minRange, defaultRew, r, g, b int) (int, int, int) {
	rgDelta := r - g
	gbDelta := g - b
	rbDelta := r - b

	if rgDelta < 0 { rgDelta = -rgDelta }
	if gbDelta < 0 { gbDelta = -gbDelta }
	if rbDelta < 0 { rbDelta = -rbDelta }

	minDelta := max(max(rgDelta, gbDelta), rbDelta)
	if minDelta < minRange {
		return r + defaultRew, g + defaultRew, b + defaultRew
	}

	if b > g && b > r {
		b += awards[2]
	} else if r > b && r > g {
		r += awards[0]
	} else if g > b && g > r {
		g += awards[1]
	}

	return r, g, b
}

func format4bitCode(code int) string {
	return fmt.Sprintf("\x1b[%dm", code)
}

func format8bitCode(code int) string {
	return fmt.Sprintf("\x1b[38;5;%dm", code)
}

func format24bitCode(r, g, b int)string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

func channelSplit(c color.Color) (int, int, int) {
	r, g, b, a := c.RGBA()
	a8uint := a >> 8
	r8 := int(r >> 8 * a8uint / 255)
	g8 := int(g >> 8 * a8uint / 255)
	b8 := int(b >> 8 * a8uint / 255)

	return r8, g8, b8
}

func default3BitColorMapperFactory(
	opts ColorMapper3BitOptions,
) func(LuminosityProvider LuminosityProvider, x, y int) (int, string) {

	return func(lumProv LuminosityProvider, x, y int) (int, string) {
		r8, g8, b8 := channelSplit(lumProv.At(x, y))
		lum := lumProv.LuminosityAt(x, y)

		code := 0

		// Check if the pixel is too dark or too bright and just assign it to black/white without doing further calculations
		if lum <= opts.BlackLumUpper {
			code = 30
			return code, format4bitCode(code)
		} else if lum >= opts.WhiteLumLower {
			code = 37
			return code, format4bitCode(code)
		}
		
		if opts.DoReward {
			// Then give additive bonuses to brightest, second brightest and third brightest channels, favouring blue, then red, then green.
			r8, g8, b8 = giveRewards(opts.ColorRewards[:], opts.ColorRewardMinRange, opts.DefaultReward, r8, g8, b8)
		}

		if r8 >= opts.ColorThresholds[0] {
			code |= 0b001
		}

		if g8 >= opts.ColorThresholds[1] {
			code |= 0b010
		}

		if b8 >= opts.ColorThresholds[2] {
			code |= 0b100
		}

		code += 30
		return code, format4bitCode(code)
	}
}

func default4BitColorMapperFactory(opts ColorMapper4BitOptions) func(LuminosityProvider, int, int) (int, string) {

	return func(lumProv LuminosityProvider, x, y int) (int, string) {
		r8, g8, b8 := channelSplit(lumProv.At(x, y))
		lum := lumProv.LuminosityAt(x, y)

		code := 0

		// Check if the pixel is too dark or too bright and just assign it to black/white without doing further calculations
		if lum <= opts.BlackLumUpper {
			code = 30
			if lum >= opts.BoldBlackLumLower {
				code += 60
				return code, format4bitCode(code)
			}
			return code, format4bitCode(code)
		} else if lum >= opts.WhiteLumLower {
			code = 37
			if lum >= opts.BoldWhiteLumLower {
				code += 60
			}
			return code, format4bitCode(code)
		}
		
		if opts.DoReward {
			// Then give additive bonuses to brightest, second brightest and third brightest channels, favouring blue, then red, then green.
			r8, g8, b8 = giveRewards(opts.ColorRewards[:], opts.ColorRewardMinRange, opts.DefaultReward, r8, g8, b8)
		}

		if r8 >= opts.ColorThresholds[0] {
			code |= 0b001
		}

		if g8 >= opts.ColorThresholds[1] {
			code |= 0b010
		}

		if b8 >= opts.ColorThresholds[2] {
			code |= 0b100
		}


		if lum >= opts.BoldColoredLumLower {
			code += 90
		} else {
			code += 30
		}

		return code, format4bitCode(code)
	}
}

func populateSteps(dest []int, rule [3]int) {
	dest[0] = rule[0]

	for i := 1; i < len(dest); i++ {
		dest[i] = rule[1] + rule[2] * i
	}
}

func default8BitColorMapperFactory(opts ColorMapper8BitOptions) func(LuminosityProvider, int, int) (int, string) {

	rSteps, gSteps, bSteps, greySteps := [6]int{}, [6]int{}, [6]int{}, [24]int{}
	populateSteps(rSteps[:], opts.rStep)
	populateSteps(gSteps[:], opts.gStep)
	populateSteps(bSteps[:], opts.bStep)
	populateSteps(greySteps[:], opts.greyStep)

	return func(lumProv LuminosityProvider, x, y int) (int, string) {
		r8, g8, b8 := channelSplit(lumProv.At(x, y))

		// Map to 6x6x6 cube
		r8dist := r8 - opts.rStep[1]
		g8dist := g8 - opts.gStep[1]
		b8dist := b8 - opts.bStep[1]

		r6 := min(5, max(0, r8dist / opts.rStep[2] + 1))
		g6 := min(5, max(0, g8dist / opts.gStep[2] + 1))
		b6 := min(5, max(0, b8dist / opts.bStep[2] + 1))

		// Map grey to grey steps
		channel8sum := r8 + b8 + g8
		avgChannel8dist := channel8sum - 3 * opts.greyStep[1]
		grey24 := min(23, max(0, avgChannel8dist / 3 * opts.greyStep[2] + 1))

		// See if the colored cube or grey is closer

		// Compute the terminal RGB values from each candidate
		closestR, closestG, closestB := rSteps[r6], gSteps[g6], bSteps[b6]

		// Compute the terminal grey value
		closestGrey := greySteps[grey24]

		rDist, gDist, bDist := closestR - r6, closestG - g6, closestB - b6
		cubeDist := rDist * rDist + gDist * gDist + bDist * bDist
		grayDist := 3 * (closestGrey-grey24) * (closestGrey-grey24)

		if grayDist < cubeDist {

			greyCode := 232 + grey24

			return greyCode, format8bitCode(greyCode)
		}

		cubeCode := 16 + (36 * r6) + (6 * g6) + b6
		return cubeCode, format8bitCode(cubeCode)
	}
}

func default24BitColorMapperFactory() func(LuminosityProvider, int, int) (int, string) {
	return func(lumProv LuminosityProvider, x, y int) (int, string) {
		r8, g8, b8 := channelSplit(lumProv.At(x, y))

		return r8 << 16 | g8 << 8 | b8, format24bitCode(r8, g8, b8)
	}
}
