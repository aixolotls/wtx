package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
	visibleWidth := lipgloss.Width(s)
	if visibleWidth > width {
		if width <= 3 {
			return truncateToWidth(s, width)
		}
		return truncateToWidth(s, width-3) + "..."
	}
	if visibleWidth < width {
		return s + strings.Repeat(" ", width-visibleWidth)
	}
	return s
}

func truncateToWidth(s string, maxWidth int) string {
	var result strings.Builder
	currentWidth := 0
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		if runes[i] == '\x1b' {
			start := i
			for i < len(runes) && runes[i] != 'm' && runes[i] != '\\' {
				i++
			}
			if i < len(runes) {
				i++
			}
			result.WriteString(string(runes[start:i]))
			continue
		}
		runeWidth := lipgloss.Width(string(runes[i]))
		if currentWidth+runeWidth > maxWidth {
			break
		}
		result.WriteRune(runes[i])
		currentWidth += runeWidth
		i++
	}
	return result.String()
}
