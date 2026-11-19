package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVisualColumn(t *testing.T) {
	tests := []struct {
		line     string
		n, width int
		want     int
	}{
		{"abc", 0, 4, 0},
		{"abc", 2, 4, 2},
		{"\tx", 1, 4, 4},
		{"\tx", 2, 4, 5},
		{"ab\tx", 3, 4, 4},
		{"ab\tx", 3, 8, 8},
		{"😀a", 2, 4, 1}, // the emoji is two code units
		{"😀a", 3, 4, 2},
		{"é\t", 2, 4, 4},
		{"abc", 99, 4, 3},
	}
	for _, tt := range tests {
		if got := visualColumn(tt.line, tt.n, tt.width); got != tt.want {
			t.Errorf("visualColumn(%q, %d, %d) = %d, want %d", tt.line, tt.n, tt.width, got, tt.want)
		}
	}
}

func TestSpacesToTabStop(t *testing.T) {
	for _, tt := range []struct{ col, width, want int }{{0, 4, 4}, {1, 4, 3}, {4, 4, 4}, {7, 8, 1}, {3, 0, 1}} {
		if got := spacesToTabStop(tt.col, tt.width); got != tt.want {
			t.Errorf("spacesToTabStop(%d, %d) = %d, want %d", tt.col, tt.width, got, tt.want)
		}
	}
}

func TestIndent(t *testing.T) {
	if indentUnit(true, 2) != "  " || indentUnit(false, 2) != "\t" {
		t.Error("indentUnit")
	}
	for _, tt := range []struct {
		line  string
		width int
		want  int
	}{
		{"\tx", 4, 1},
		{"    x", 4, 4},
		{"      x", 4, 4},
		{"  x", 4, 2},
		{"  \tx", 4, 3},
		{"x", 4, 0},
		{"", 4, 0},
	} {
		if got := unindentLength(tt.line, tt.width); got != tt.want {
			t.Errorf("unindentLength(%q, %d) = %d, want %d", tt.line, tt.width, got, tt.want)
		}
	}
}

func TestPositionText(t *testing.T) {
	for _, tt := range []struct {
		line, col, chars, lines int
		want                    string
	}{
		{1, 1, 0, 1, "Ln 1, Col 1"},
		{3, 7, 1, 1, "Ln 3, Col 7 (1 char selected)"},
		{3, 7, 12, 1, "Ln 3, Col 7 (12 chars selected)"},
		{9, 1, 40, 3, "Ln 9, Col 1 (3 lines, 40 chars selected)"},
	} {
		if got := positionText(tt.line, tt.col, tt.chars, tt.lines); got != tt.want {
			t.Errorf("positionText = %q, want %q", got, tt.want)
		}
	}
}

func TestPlainText(t *testing.T) {
	if got := plainText("a b c d"); got != "a\nb\nc d" {
		t.Errorf("plainText = %q", got)
	}
	if countChars("a 😀") != 3 {
		t.Error("countChars")
	}
}

func TestDisplayPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := displayPath(filepath.Join(home, "notes.txt")); got != "~/notes.txt" {
		t.Errorf("displayPath = %q", got)
	}
	if got := displayPath("/etc/hosts"); got != "/etc/hosts" {
		t.Errorf("displayPath = %q", got)
	}
	if got := displayPath(home + "x/y"); got != home+"x/y" {
		t.Errorf("a sibling of home: %q", got)
	}
}

func TestMatchCount(t *testing.T) {
	for _, c := range []struct {
		current, n int
		more       bool
		want       string
	}{
		{0, 0, false, "No matches"},
		{0, 1, false, "1 match"},
		{0, 3, false, "3 matches"},
		{0, 2000, true, "2000+ matches"},
		{2, 3, false, "2 of 3"},
		{5, 2000, true, "5 of 2000+"},
	} {
		if got := matchCount(c.current, c.n, c.more); got != c.want {
			t.Errorf("matchCount(%d, %d, %v) = %q, want %q", c.current, c.n, c.more, got, c.want)
		}
	}
}

func TestUTF16Prefix(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want int
	}{
		{"abc", 0, 0},
		{"abc", 2, 2},
		{"abc", 9, 3},
		{"é!", 1, 2},    // é is one unit, two bytes
		{"😀!", 2, 4},    // the emoji is two units, four bytes
		{"\xffa", 1, 1}, // a bad byte is one unit, one byte
	}
	for _, tt := range tests {
		if got := utf16Prefix(tt.s, tt.n); got != tt.want {
			t.Errorf("utf16Prefix(%q, %d) = %d, want %d", tt.s, tt.n, got, tt.want)
		}
	}
}

func TestPlural(t *testing.T) {
	for n, want := range map[int]string{0: "0 lines", 1: "1 line", 2: "2 lines"} {
		if got := plural(n, "line", "lines"); got != want {
			t.Errorf("plural(%d) = %q, want %q", n, got, want)
		}
	}
}
