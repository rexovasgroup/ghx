package iostreams

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mgutz/ansi"
)

const (
	// NoTheme indicates that no terminal theme is applied.
	NoTheme = "none"
	// DarkTheme indicates a dark terminal background theme.
	DarkTheme = "dark"
	// LightTheme indicates a light terminal background theme.
	LightTheme     = "light"
	highlightStyle = "black:yellow"
)

// Special cases like darkThemeTableHeader / lightThemeTableHeader are necessary when using color and modifiers
// (bold, underline, dim) because ansi.ColorFunc requires a foreground color and resets formats.
var (
	magenta               = ansi.ColorFunc("magenta")
	cyan                  = ansi.ColorFunc("cyan")
	red                   = ansi.ColorFunc("red")
	yellow                = ansi.ColorFunc("yellow")
	blue                  = ansi.ColorFunc("blue")
	green                 = ansi.ColorFunc("green")
	gray                  = ansi.ColorFunc("black+h")
	bold                  = ansi.ColorFunc("default+b")
	cyanBold              = ansi.ColorFunc("cyan+b")
	greenBold             = ansi.ColorFunc("green+b")
	highlightStart        = ansi.ColorCode(highlightStyle)
	highlight             = ansi.ColorFunc(highlightStyle)
	darkThemeMuted        = ansi.ColorFunc("white+d")
	darkThemeTableHeader  = ansi.ColorFunc("white+du")
	lightThemeMuted       = ansi.ColorFunc("black+h")
	lightThemeTableHeader = ansi.ColorFunc("black+hu")
	noThemeTableHeader    = ansi.ColorFunc("default+u")

	gray256 = func(t string) string {
		return fmt.Sprintf("\x1b[%d;5;%dm%s\x1b[0m", 38, 242, t)
	}
)

// ColorScheme controls how text is colored based upon terminal capabilities and user preferences.
type ColorScheme struct {
	// Enabled is whether color is used at all.
	Enabled bool
	// EightBitColor is whether the terminal supports 8-bit, 256 colors.
	EightBitColor bool
	// TrueColor is whether the terminal supports 24-bit, 16 million colors.
	TrueColor bool
	// Accessible is whether colors must be base 16 colors that users can customize in terminal preferences.
	Accessible bool
	// ColorLabels is whether labels are colored based on their truecolor RGB hex color.
	ColorLabels bool
	// Theme is the terminal background color theme used to contextually color text for light, dark, or none at all.
	Theme string
}

// Bold applies bold formatting to the given text.
func (c *ColorScheme) Bold(t string) string {
	if !c.Enabled {
		return t
	}
	return bold(t)
}

// Boldf applies bold formatting to a formatted string.
func (c *ColorScheme) Boldf(t string, args ...interface{}) string {
	return c.Bold(fmt.Sprintf(t, args...))
}

// Muted applies a muted color to the given text based on the terminal theme.
func (c *ColorScheme) Muted(t string) string {
	// Fallback to previous logic if accessible colors preview is disabled.
	if !c.Accessible {
		return c.Gray(t)
	}

	// Muted text is only stylized if color is enabled.
	if !c.Enabled {
		return t
	}

	switch c.Theme {
	case LightTheme:
		return lightThemeMuted(t)
	case DarkTheme:
		return darkThemeMuted(t)
	default:
		return t
	}
}

// Mutedf applies a muted color to a formatted string.
func (c *ColorScheme) Mutedf(t string, args ...interface{}) string {
	return c.Muted(fmt.Sprintf(t, args...))
}

// Red applies red color to the given text.
func (c *ColorScheme) Red(t string) string {
	if !c.Enabled {
		return t
	}
	return red(t)
}

// Redf applies red color to a formatted string.
func (c *ColorScheme) Redf(t string, args ...interface{}) string {
	return c.Red(fmt.Sprintf(t, args...))
}

// Yellow applies yellow color to the given text.
func (c *ColorScheme) Yellow(t string) string {
	if !c.Enabled {
		return t
	}
	return yellow(t)
}

// Yellowf applies yellow color to a formatted string.
func (c *ColorScheme) Yellowf(t string, args ...interface{}) string {
	return c.Yellow(fmt.Sprintf(t, args...))
}

// Green applies green color to the given text.
func (c *ColorScheme) Green(t string) string {
	if !c.Enabled {
		return t
	}
	return green(t)
}

// Greenf applies green color to a formatted string.
func (c *ColorScheme) Greenf(t string, args ...interface{}) string {
	return c.Green(fmt.Sprintf(t, args...))
}

// GreenBold applies green color with bold formatting to the given text.
func (c *ColorScheme) GreenBold(t string) string {
	if !c.Enabled {
		return t
	}
	return greenBold(t)
}

// Deprecated: Use Muted instead for thematically contrasting color.
func (c *ColorScheme) Gray(t string) string {
	if !c.Enabled {
		return t
	}
	if c.EightBitColor {
		return gray256(t)
	}
	return gray(t)
}

// Deprecated: Use Mutedf instead for thematically contrasting color.
func (c *ColorScheme) Grayf(t string, args ...interface{}) string {
	return c.Gray(fmt.Sprintf(t, args...))
}

// Magenta applies magenta color to the given text.
func (c *ColorScheme) Magenta(t string) string {
	if !c.Enabled {
		return t
	}
	return magenta(t)
}

// Magentaf applies magenta color to a formatted string.
func (c *ColorScheme) Magentaf(t string, args ...interface{}) string {
	return c.Magenta(fmt.Sprintf(t, args...))
}

// Cyan applies cyan color to the given text.
func (c *ColorScheme) Cyan(t string) string {
	if !c.Enabled {
		return t
	}
	return cyan(t)
}

// Cyanf applies cyan color to a formatted string.
func (c *ColorScheme) Cyanf(t string, args ...interface{}) string {
	return c.Cyan(fmt.Sprintf(t, args...))
}

// CyanBold applies cyan color with bold formatting to the given text.
func (c *ColorScheme) CyanBold(t string) string {
	if !c.Enabled {
		return t
	}
	return cyanBold(t)
}

// Blue applies blue color to the given text.
func (c *ColorScheme) Blue(t string) string {
	if !c.Enabled {
		return t
	}
	return blue(t)
}

// Bluef applies blue color to a formatted string.
func (c *ColorScheme) Bluef(t string, args ...interface{}) string {
	return c.Blue(fmt.Sprintf(t, args...))
}

// SuccessIcon returns the colored success icon (✓).
func (c *ColorScheme) SuccessIcon() string {
	return c.SuccessIconWithColor(c.Green)
}

// SuccessIconWithColor returns the success icon (✓) using the given color function.
func (c *ColorScheme) SuccessIconWithColor(colo func(string) string) string {
	return colo("✓")
}

// WarningIcon returns the colored warning icon (!).
func (c *ColorScheme) WarningIcon() string {
	return c.Yellow("!")
}

// FailureIcon returns the colored failure icon (X).
func (c *ColorScheme) FailureIcon() string {
	return c.FailureIconWithColor(c.Red)
}

// FailureIconWithColor returns the failure icon (X) using the given color function.
func (c *ColorScheme) FailureIconWithColor(colo func(string) string) string {
	return colo("X")
}

// HighlightStart returns the ANSI escape code to begin highlighted text.
func (c *ColorScheme) HighlightStart() string {
	if !c.Enabled {
		return ""
	}

	return highlightStart
}

// Highlight applies highlight formatting to the given text.
func (c *ColorScheme) Highlight(t string) string {
	if !c.Enabled {
		return t
	}

	return highlight(t)
}

// Reset returns the ANSI escape code to reset all formatting.
func (c *ColorScheme) Reset() string {
	if !c.Enabled {
		return ""
	}

	return ansi.Reset
}

// ColorFromString returns the color function corresponding to the given color name.
func (c *ColorScheme) ColorFromString(s string) func(string) string {
	s = strings.ToLower(s)
	var fn func(string) string
	switch s {
	case "bold":
		fn = c.Bold
	case "red":
		fn = c.Red
	case "yellow":
		fn = c.Yellow
	case "green":
		fn = c.Green
	case "gray":
		fn = c.Muted
	case "magenta":
		fn = c.Magenta
	case "cyan":
		fn = c.Cyan
	case "blue":
		fn = c.Blue
	default:
		fn = func(s string) string {
			return s
		}
	}

	return fn
}

// Label stylizes text based on label's RGB hex color.
func (c *ColorScheme) Label(hex string, x string) string {
	if !c.Enabled || !c.TrueColor || !c.ColorLabels || len(hex) != 6 {
		return x
	}

	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", r, g, b, x)
}

// TableHeader applies table header styling to the given text based on the terminal theme.
func (c *ColorScheme) TableHeader(t string) string {
	// Table headers are only stylized if color is enabled including underline modifier.
	if !c.Enabled {
		return t
	}

	switch c.Theme {
	case DarkTheme:
		return darkThemeTableHeader(t)
	case LightTheme:
		return lightThemeTableHeader(t)
	default:
		return noThemeTableHeader(t)
	}
}
