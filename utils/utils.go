package utils

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// IsDebugEnabled reports whether debug mode is active.
func IsDebugEnabled() (bool, string) {
	debugValue, isDebugSet := os.LookupEnv("GH_DEBUG")
	legacyDebugValue := os.Getenv("DEBUG")

	if !isDebugSet {
		switch legacyDebugValue {
		case "true", "1", "yes", "api":
			return true, legacyDebugValue
		default:
			return false, legacyDebugValue
		}
	}

	switch debugValue {
	case "false", "0", "no", "":
		return false, debugValue
	default:
		return true, debugValue
	}
}

// TerminalSize returns the width and height of the terminal.
var TerminalSize = func(w interface{}) (int, int, error) {
	if f, isFile := w.(*os.File); isFile {
		return term.GetSize(int(f.Fd()))
	}

	return 0, 0, fmt.Errorf("%v is not a file", w)
}
