package config

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ThemeSpec is an editor colour theme as the settings file stores it.
// Colours are "#rrggbb" (or "#rgb"). Background and Foreground must be
// set; the rest may be left empty, and are then made from those two.
type ThemeSpec struct {
	Name          string `json:"name"`
	Background    string `json:"background"`
	Foreground    string `json:"foreground"`
	CurrentLine   string `json:"current_line,omitempty"`
	Selection     string `json:"selection,omitempty"`
	SelectionText string `json:"selection_text,omitempty"`
	Gutter        string `json:"gutter,omitempty"`
	GutterText    string `json:"gutter_text,omitempty"`
	GutterCurrent string `json:"gutter_current,omitempty"`
	Match         string `json:"match,omitempty"`
	MatchText     string `json:"match_text,omitempty"`
}

// Theme is a resolved theme: every colour set, as "#rrggbb".
type Theme struct {
	Name          string
	Background    string
	Foreground    string
	CurrentLine   string // the background of the line the cursor is on
	Selection     string
	SelectionText string
	Gutter        string // the line numbers' background
	GutterText    string // the line numbers
	GutterCurrent string // the number of the line the cursor is on
	Match         string // the background of search matches
	MatchText     string
}

// DefaultTheme is the id of the theme used unless another is chosen.
const DefaultTheme = "freya"

// The Solarized colours (Ethan Schoonover's palette).
const (
	solBase03 = "#002b36"
	solBase02 = "#073642"
	solBase01 = "#586e75"
	solBase00 = "#657b83"
	solBase0  = "#839496"
	solBase1  = "#93a1a1"
	solBase2  = "#eee8d5"
	solBase3  = "#fdf6e3"
	solViolet = "#6c71c4"
)

// BuiltinThemes are the themes the editor comes with. They are the theme
// list until the settings file has one of its own, and can be put back
// from the Settings dialog.
var BuiltinThemes = map[string]ThemeSpec{
	"freya": {
		Name:          "Freya",
		Background:    "#2a2a2a",
		Foreground:    "#dcdccc",
		CurrentLine:   "#202020",
		Selection:     "#595959",
		SelectionText: "#dcdccc",
		Gutter:        "#1f1f1f",
		GutterText:    "#595959",
		GutterCurrent: "#595959",
		Match:         "#295e8e",
		MatchText:     "#ffffff",
	},
	"solarized-light": {
		Name:          "Solarized Light",
		Background:    solBase3,
		Foreground:    solBase00,
		CurrentLine:   solBase2,
		Selection:     solBase1,
		SelectionText: solBase3,
		Gutter:        solBase2,
		GutterText:    solBase1,
		GutterCurrent: solBase01,
		Match:         solViolet,
		MatchText:     solBase3,
	},
	"solarized-dark": {
		Name:          "Solarized Dark",
		Background:    solBase03,
		Foreground:    solBase0,
		CurrentLine:   solBase02,
		Selection:     solBase01,
		SelectionText: solBase2,
		Gutter:        solBase02,
		GutterText:    solBase01,
		GutterCurrent: solBase1,
		Match:         solViolet,
		MatchText:     solBase2,
	},
}

// Resolve parses the theme's colours, making those left empty from the
// background and foreground.
func (s ThemeSpec) Resolve() (Theme, error) {
	t := Theme{Name: s.Name}
	var errs []string
	parse := func(field, v string, dst *string, fallback string) {
		if v == "" && fallback != "" {
			*dst = fallback
			return
		}
		c, ok := ParseColor(v)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s %q is not a colour", field, v))
			*dst = fallback
			return
		}
		*dst = HexColor(c)
	}
	parse("background", s.Background, &t.Background, "")
	parse("foreground", s.Foreground, &t.Foreground, "")
	if t.Background == "" {
		t.Background = "#ffffff"
	}
	if t.Foreground == "" {
		t.Foreground = "#000000"
	}
	bg, _ := ParseColor(t.Background)
	fg, _ := ParseColor(t.Foreground)
	mix := func(f float64) string { return HexColor(Mix(bg, fg, f)) }
	parse("current_line", s.CurrentLine, &t.CurrentLine, mix(0.08))
	parse("selection", s.Selection, &t.Selection, mix(0.3))
	parse("selection_text", s.SelectionText, &t.SelectionText, t.Foreground)
	parse("gutter", s.Gutter, &t.Gutter, t.CurrentLine)
	parse("gutter_text", s.GutterText, &t.GutterText, mix(0.45))
	parse("gutter_current", s.GutterCurrent, &t.GutterCurrent, t.Foreground)
	parse("match", s.Match, &t.Match, solViolet)
	parse("match_text", s.MatchText, &t.MatchText, t.Background)
	if len(errs) > 0 {
		return t, fmt.Errorf("theme %q: %s", s.Name, strings.Join(errs, "; "))
	}
	return t, nil
}

// Colours returns the theme's colours.
func (t Theme) Colours() []string {
	return []string{t.Background, t.Foreground, t.CurrentLine, t.Selection, t.SelectionText,
		t.Gutter, t.GutterText, t.GutterCurrent, t.Match, t.MatchText}
}

// ParseColor parses "#rrggbb" or "#rgb" as 0xRRGGBB.
func ParseColor(s string) (uint32, bool) {
	hex, ok := strings.CutPrefix(strings.TrimSpace(s), "#")
	if !ok {
		return 0, false
	}
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return 0, false
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

// HexColor formats 0xRRGGBB as "#rrggbb".
func HexColor(c uint32) string { return fmt.Sprintf("#%06x", c&0xffffff) }

// Mix blends colour a towards b by f (0..1); colours are 0xRRGGBB.
func Mix(a, b uint32, f float64) uint32 {
	ch := func(s uint) uint32 {
		return uint32(float64(a>>s&0xff)*(1-f)+float64(b>>s&0xff)*f+0.5) << s
	}
	return ch(16) | ch(8) | ch(0)
}

// ThemeIDs returns the ids of the settings' themes, ordered by name.
func (c *Config) ThemeIDs() []string {
	themes := c.Themes
	ids := slices.Collect(maps.Keys(themes))
	slices.SortFunc(ids, func(a, b string) int {
		if c := strings.Compare(strings.ToLower(themes[a].Name), strings.ToLower(themes[b].Name)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	return ids
}

// Theme returns the editor theme the settings choose. One that is missing
// or does not resolve gives the built-in default.
func (c *Config) Theme() Theme {
	if spec, ok := c.Themes[c.EditorTheme]; ok {
		if th, err := spec.Resolve(); err == nil {
			return th
		}
	}
	th, _ := BuiltinThemes[DefaultTheme].Resolve()
	return th
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// NewThemeID returns an unused id for a theme called name.
func NewThemeID(themes map[string]ThemeSpec, name string) string {
	base := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if base == "" {
		base = "theme"
	}
	id := base
	for n := 2; ; n++ {
		if _, taken := themes[id]; !taken {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
}
