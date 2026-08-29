// Package theme holds the colours the interface draws with, taken from the
// Catppuccin Macchiato flavour (https://catppuccin.com/palette/).
package theme

import "fmt"

// RGB is a colour with 8 bits per channel.
type RGB struct{ R, G, B uint8 }

// Catppuccin Macchiato. Only the colours this program actually draws with are
// listed; add more from the palette rather than inventing shades.
var (
	Green  = RGB{0xa6, 0xda, 0x95}
	Yellow = RGB{0xee, 0xd4, 0x9f}
	Peach  = RGB{0xf5, 0xa9, 0x7f}
	Maroon = RGB{0xee, 0x99, 0xa0}
	Red    = RGB{0xed, 0x87, 0x96}
)

// Hex renders the colour as "#rrggbb", for libraries that take one.
func (c RGB) Hex() string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// ANSI renders the colour as a truecolor escape sequence.
func (c RGB) ANSI() string { return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B) }

// Lerp blends between two colours. t outside 0..1 is clamped.
func Lerp(a, b RGB, t float64) RGB {
	switch {
	case t <= 0:
		return a
	case t >= 1:
		return b
	}
	mix := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return RGB{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B)}
}

// Ramp blends along a sequence of colours, so a gradient can be built from
// palette entries instead of passing through shades the palette does not have.
func Ramp(stops []RGB, t float64) RGB {
	switch {
	case len(stops) == 0:
		return RGB{}
	case len(stops) == 1 || t <= 0:
		return stops[0]
	case t >= 1:
		return stops[len(stops)-1]
	}
	span := 1.0 / float64(len(stops)-1)
	i := int(t / span)
	if i >= len(stops)-1 {
		i = len(stops) - 2
	}
	return Lerp(stops[i], stops[i+1], (t-float64(i)*span)/span)
}
