package ui

import "strings"

type Styles struct {
	Header           func(string) string
	Normal           func(string) string
	Selected         func(string) string
	Disabled         func(string) string
	DisabledSelected func(string) string
	Secondary        func(string) string
}

func PadOrTrim(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > width {
		if width <= 3 {
			return string(r[:width])
		}
		return string(r[:width-3]) + "..."
	}
	if len(r) < width {
		return s + strings.Repeat(" ", width-len(r))
	}
	return s
}
