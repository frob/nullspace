package response

import (
	"context"
	"strconv"
	"strings"
)

const (
	ansiBold     = "\033[1m"
	ansiBoldRed  = "\033[1;31m"
	ansiRed      = "\033[31m"
	ansiYellow   = "\033[33m"
	ansiCyan     = "\033[36m"
	ansiReset    = "\033[0m"
)

// ANSIFormatter serializes response data as plain text with ANSI escape
// codes for color and emphasis. Useful for TUIs, CLI tools, and terminal
// debugging.
type ANSIFormatter struct{}

func (f *ANSIFormatter) Name() string        { return "ansi" }
func (f *ANSIFormatter) ContentType() string { return "text/plain; charset=utf-8" }

func (f *ANSIFormatter) Format(_ context.Context, resp *Response) ([]byte, error) {
	r := renderText(resp.Data)
	return renderANSI(r), nil
}

// renderANSI writes the textRender with ANSI escape codes.
func renderANSI(r textRender) []byte {
	var b strings.Builder

	for _, f := range r.Fields {
		b.WriteString(ansiBold)
		b.WriteString(f.Key)
		b.WriteString(ansiReset)
		b.WriteString(": ")
		b.WriteString(colorizeValue(f.Key, f.Value))
		b.WriteString("\n")
	}

	if r.Body != "" {
		if len(r.Fields) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(colorizeBody(r))
		b.WriteString("\n")
	}

	return []byte(b.String())
}

// colorizeValue applies color to specific field values.
func colorizeValue(key, value string) string {
	lower := strings.ToLower(key)

	if lower == "error" {
		return ansiBoldRed + value + ansiReset
	}

	if lower == "status" {
		if n, err := strconv.Atoi(value); err == nil {
			return colorizeStatus(n, value)
		}
	}

	return value
}

// colorizeStatus applies color based on HTTP status code ranges.
func colorizeStatus(code int, text string) string {
	switch {
	case code >= 500:
		return ansiRed + text + ansiReset
	case code >= 400:
		return ansiYellow + text + ansiReset
	default:
		return text
	}
}

// colorizeBody applies ANSI codes to body text. List item indices get
// cyan coloring.
func colorizeBody(r textRender) string {
	lines := strings.Split(r.Body, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		// Color list indices like [1], [2], etc.
		if len(line) > 0 && line[0] == '[' {
			if idx := strings.Index(line, "]"); idx > 0 {
				b.WriteString(ansiCyan)
				b.WriteString(line[:idx+1])
				b.WriteString(ansiReset)
				b.WriteString(line[idx+1:])
				continue
			}
		}
		b.WriteString(line)
	}

	return b.String()
}

// ansiFieldCount counts ANSI escape sequences for testing.
func ansiFieldCount(data []byte, seq string) int {
	return strings.Count(string(data), seq)
}

// ansiStrip removes all ANSI escape codes from a byte slice.
func ansiStrip(data []byte) string {
	s := string(data)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip to end of escape sequence (letter).
			j := i + 2
			for j < len(s) && !isANSITerminator(s[j]) {
				j++
			}
			if j < len(s) {
				i = j // skip past terminator
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isANSITerminator(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}
