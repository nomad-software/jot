package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// The helpers in this file work on Go strings alone, so that they can be
// tested without Qt.

// utf16Prefix returns how many bytes of s hold its first n UTF-16 code
// units: Qt counts positions in a line in code units.
func utf16Prefix(s string, n int) int {
	units := 0
	for i, r := range s {
		if units >= n {
			return i
		}
		units += utf16.RuneLen(r) // an invalid byte is U+FFFD here, one unit
	}
	return len(s)
}

// visualColumn returns the column, counting from 0, at which the character
// n UTF-16 code units into line is shown: tabs reach the next tab stop.
func visualColumn(line string, n, tabWidth int) int {
	col := 0
	for _, r := range line[:utf16Prefix(line, n)] {
		if r == '\t' && tabWidth > 0 {
			col += tabWidth - col%tabWidth
		} else {
			col++
		}
	}
	return col
}

// spacesToTabStop returns how many spaces take the cursor from the visual
// column col to the next tab stop.
func spacesToTabStop(col, tabWidth int) int {
	if tabWidth <= 0 {
		return 1
	}
	return tabWidth - col%tabWidth
}

// indentUnit returns what one level of indentation is: a tab, or tabWidth
// spaces.
func indentUnit(insertSpaces bool, tabWidth int) string {
	if insertSpaces {
		return strings.Repeat(" ", max(tabWidth, 1))
	}
	return "\t"
}

// unindentLength returns how many characters (all ASCII, so also UTF-16
// code units) to remove from the start of line to take away one level of
// indentation: a tab, or up to tabWidth spaces, or the spaces before a tab.
func unindentLength(line string, tabWidth int) int {
	n := 0
	for n < len(line) && n < max(tabWidth, 1) {
		switch line[n] {
		case ' ':
			n++
			continue
		case '\t':
			return n + 1
		}
		break
	}
	return n
}

// positionText is the status bar's line and column, with what is selected.
// lines is how many lines the selection touches and chars how many
// characters it holds (0 for none).
func positionText(line, col, chars, lines int) string {
	s := fmt.Sprintf("Ln %d, Col %d", line, col)
	switch {
	case chars == 0:
	case lines > 1:
		s += fmt.Sprintf(" (%s, %s selected)", plural(lines, "line", "lines"), plural(chars, "char", "chars"))
	default:
		s += fmt.Sprintf(" (%s selected)", plural(chars, "char", "chars"))
	}
	return s
}

// plural returns n and the word for one or for many, as n is.
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// matchCount is the find bar's count of a search's matches: n of them,
// more than that if more, and which is selected (from 1; 0 for none).
func matchCount(current, n int, more bool) string {
	plus := ""
	if more {
		plus = "+"
	}
	switch {
	case n == 0:
		return "No matches"
	case current > 0:
		return fmt.Sprintf("%d of %d%s", current, n, plus)
	case n == 1 && !more:
		return "1 match"
	}
	return fmt.Sprintf("%d%s matches", n, plus)
}

// countChars counts the characters of text Qt gives: runes, with its
// paragraph and line separators each one character, as a newline is.
func countChars(text string) int { return utf8.RuneCountInString(text) }

// plainText turns the text Qt holds (its raw text, which keeps
// non-breaking spaces where toPlainText would not) into the text of a file:
// paragraph separators and line separators become "\n".
func plainText(raw string) string {
	if !strings.ContainsAny(raw, "  ") {
		return raw
	}
	return strings.NewReplacer(" ", "\n", " ", "\n").Replace(raw)
}

// displayPath shortens a path in the home folder to one starting with ~.
func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || home == "/" {
		return path
	}
	if path == home {
		return "~"
	}
	if rel, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rel
	}
	return path
}

// absPath makes a path given on the command line absolute.
func absPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
