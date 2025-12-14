package utils

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

const (
	ProgressBarWidth  = 71
	ProgressFullChar  = "█"
	ProgressEmptyChar = "░"
	DotChar           = " • "
)

// General stuff for styling the view
var (
	KeywordStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	SubtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	CheckboxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	ProgressEmpty = SubtleStyle.Render(ProgressEmptyChar)
	DotStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render(DotChar)
	MainStyle     = lipgloss.NewStyle().MarginLeft(2)

	Ramp = MakeRampStyles("#B14FFF", "#00FFA3", ProgressBarWidth)
)

// Generate a blend of colors.
func MakeRampStyles(colorA, colorB string, steps float64) (s []lipgloss.Style) {
	cA, _ := colorful.Hex(colorA)
	cB, _ := colorful.Hex(colorB)

	for i := 0.0; i < steps; i++ {
		c := cA.BlendLuv(cB, i/steps)
		s = append(s, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorToHex(c))))
	}
	return
}

// Convert a colorful.Color to a hexadecimal format.
func ColorToHex(c colorful.Color) string {
	return fmt.Sprintf("#%s%s%s", ColorFloatToHex(c.R), ColorFloatToHex(c.G), ColorFloatToHex(c.B))
}

// Helper function for converting colors to hex. Assumes a value between 0 and
// 1.
func ColorFloatToHex(f float64) (s string) {
	s = strconv.FormatInt(int64(f*255), 16)
	if len(s) == 1 {
		s = "0" + s
	}
	return
}

func Progressbar(percent float64) string {
	w := float64(ProgressBarWidth)

	fullSize := int(math.Round(w * percent))
	var fullCells string
	for i := 0; i < fullSize; i++ {
		fullCells += Ramp[i].Render(ProgressFullChar)
	}

	emptySize := int(w) - fullSize
	emptyCells := strings.Repeat(ProgressEmpty, emptySize)

	return fmt.Sprintf("%s%s %3.0f", fullCells, emptyCells, math.Round(percent*100))
}

func Checkbox(label string, checked bool) string {
	if checked {
		return CheckboxStyle.Render("[x] " + label)
	}
	return fmt.Sprintf("[ ] %s", label)
}
