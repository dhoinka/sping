package main

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"
)

func terminalSize(out io.Writer) (int, int) {
	if file, ok := out.(interface{ Fd() uintptr }); ok {
		if width, height, err := term.GetSize(int(file.Fd())); err == nil && width > 0 && height > 0 {
			return width, height
		}
	}
	return 80, 24
}

var colorCode = regexp.MustCompile(`\x1b\[[0-9;]*m`)
var displayToken = regexp.MustCompile(`\x1b\[[0-9;]*m|.`)

func visibleWidth(text string) int {
	return len([]rune(colorCode.ReplaceAllString(text, "")))
}

func clipLine(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if visibleWidth(text) <= width {
		return text
	}
	limit := width
	if width > 1 {
		limit--
	}
	var clipped strings.Builder
	used := 0
	for _, token := range displayToken.FindAllString(text, -1) {
		if strings.HasPrefix(token, "\x1b[") {
			clipped.WriteString(token)
			continue
		}
		if used == limit {
			break
		}
		clipped.WriteString(token)
		used++
	}
	if width > 1 {
		clipped.WriteString("…")
	}
	if colorCode.MatchString(text) {
		clipped.WriteString("\x1b[0m")
	}
	return clipped.String()
}

func (t *tracker) twoLines(host string, width, limit int, scale time.Duration, color bool) string {
	width = max(1, width)
	status := t.status(color)
	// Give statistics priority over the hostname in narrow terminals.
	room := width - visibleWidth(status) - 3
	if room > 0 {
		status = clipLine(host, room) + " | " + status
	}
	chartWidth := width
	if limit > 0 {
		chartWidth = min(chartWidth, limit)
	}
	return clipLine(status, width) + "\r\n\x1b[2K" + t.chart(chartWidth, scale, color)
}

func drawScreen(out io.Writer, t *tracker, host string, limit int, scale time.Duration, color bool) {
	width, _ := terminalSize(out)
	// Leave one column free to avoid automatic wrapping at the terminal edge.
	fmt.Fprint(out, "\r\x1b[1A\x1b[2K", t.twoLines(host, max(1, width-1), limit, scale, color))
}
