// The asciiart package implements the logic for generating ascii art from some image.
// By default, the package supports .png, .jpg, .jpeg and .gif. See ConvertBytes() and ConvertReader()
// To support other image formats, either use Convert() instead or import your custom decoders like so:
/*
import (
	... <other imports>

	_ "mycustomdecoder/mycustomformat" // Here is your custom file format
	
	...
)
*/
// Start by calling New() or NewDefault(). Pass the options into the constructors (see options.go).
// While all fields are public, treat the asciiconverter struct as immutable (and thread unsafe).
package asciiart
