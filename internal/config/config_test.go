package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// colours returns every colour of a theme.
func colours(t Theme) []string {
	return []string{t.Background, t.Foreground, t.CurrentLine, t.Selection, t.SelectionText,
		t.Gutter, t.GutterText, t.GutterCurrent, t.Match, t.MatchText}
}

func write(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadMissingIsDefault(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "none.json"))
	if err != nil {
		t.Fatal(err)
	}
	if c.TabWidth != 4 || !c.ShowLineNumbers || c.EditorTheme != DefaultTheme {
		t.Errorf("defaults = %+v", c)
	}
}

func TestLoadMergesKeybindings(t *testing.T) {
	c, err := Load(write(t, `{"tab_width": 8, "keybindings": {"save": "Ctrl+Alt+S"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.TabWidth != 8 {
		t.Errorf("tab_width = %d", c.TabWidth)
	}
	if c.Keybindings["save"] != "Ctrl+Alt+S" || c.Keybindings["open"] != DefaultKeybindings["open"] {
		t.Errorf("keybindings = %v", c.Keybindings)
	}
	if DefaultKeybindings["save"] != "Ctrl+S" {
		t.Error("loading changed the defaults")
	}
}

func TestLoadReportsProblemsAndKeepsTheRest(t *testing.T) {
	c, err := Load(write(t, `{"tab_width": "wide", "font_size": 14, "window_theme": "purple", "editor_theme": "nope", "colour": 1, "keybindings": {"fly": "F1"}}`))
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{"tab_width", "window_theme", "editor_theme", "colour", "fly"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
	if strings.Count(err.Error(), "\n") > 0 {
		t.Errorf("error spans lines: %q", err)
	}
	if c.FontSize != 14 || c.TabWidth != 4 || c.WindowTheme != "system" || c.EditorTheme != DefaultTheme {
		t.Errorf("config = %+v", c)
	}
	if _, ok := c.Keybindings["fly"]; ok {
		t.Error("unknown action kept")
	}
}

func TestLoadSyntaxErrorHasLine(t *testing.T) {
	c, err := Load(write(t, "{\n\"tab_width\": 8,\n}"))
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("err = %v", err)
	}
	if c.TabWidth != 4 {
		t.Error("a broken file did not give the defaults")
	}
}

func TestNormalizeClamps(t *testing.T) {
	c, _ := Load(write(t, `{"tab_width": 100, "font_size": 1000}`))
	if c.TabWidth != MaxTabWidth || c.FontSize != MaxFontSize {
		t.Errorf("tab_width %d, font_size %v", c.TabWidth, c.FontSize)
	}
}

func TestClashes(t *testing.T) {
	// The shortcut is spelt as the first action, by name, has it, every
	// time: not as whichever a map gives first.
	for range 20 {
		got := Clashes(map[string]string{"save": "Ctrl+S", "open": "ctrl + s", "find": "Ctrl+F", "about": "", "quit": "F1", "undo": "f1"})
		want := []Clash{{"F1", []string{"quit", "undo"}}, {"ctrl + s", []string{"open", "save"}}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Clashes = %v, want %v", got, want)
		}
	}
	if got := (Clash{"Ctrl+S", []string{"save", "save_as"}}).String(); got != "Ctrl+S (save, save_as)" {
		t.Errorf("String = %q", got)
	}
	if got := Clashes(DefaultKeybindings); len(got) != 0 {
		t.Errorf("the defaults clash: %v", got)
	}
	// A file that gives an action another's shortcut is reported, and the
	// shortcuts are kept as it says.
	c, err := Load(write(t, `{"keybindings": {"open": "Ctrl+S"}}`))
	if err == nil || !strings.Contains(err.Error(), "keybindings used by more than one action: Ctrl+S (open, save)") {
		t.Errorf("err = %v", err)
	}
	if c.Keybindings["open"] != "Ctrl+S" {
		t.Errorf("keybindings = %v", c.Keybindings)
	}
}

func TestSetValuesKeepsOtherKeysAndMode(t *testing.T) {
	path := write(t, `{"tab_width": 8, "custom": [1, 2]}`)
	if err := SetValues(path, map[string]any{"font_size": 13.5}); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["tab_width"] != 8.0 || doc["font_size"] != 13.5 || doc["custom"] == nil {
		t.Errorf("file = %s", data)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", info.Mode().Perm())
	}
}

func TestSetValuesCreatesFolderAndFollowsLinks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "config.json")
	if err := SetValues(path, map[string]any{"toolbar": false}); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil || c.Toolbar {
		t.Fatalf("toolbar %v, err %v", c.Toolbar, err)
	}

	target := filepath.Join(dir, "real.json")
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := SetValues(link, map[string]any{"tab_width": 2}); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Error("the link was replaced")
	}
	if c, _ := Load(target); c.TabWidth != 2 {
		t.Error("the link's target was not written")
	}
}

func TestThemes(t *testing.T) {
	hex := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	if _, ok := BuiltinThemes[DefaultTheme]; !ok {
		t.Fatal("no default theme")
	}
	for id, spec := range BuiltinThemes {
		th, err := spec.Resolve()
		if err != nil || th.Name == "" {
			t.Errorf("%s: %v, name %q", id, err, th.Name)
		}
		for _, c := range colours(th) {
			if !hex.MatchString(c) {
				t.Errorf("%s: colour %q", id, c)
			}
		}
	}
	if ids := Default().ThemeIDs(); len(ids) != len(BuiltinThemes) || ids[0] != "freya" {
		t.Errorf("ThemeIDs = %v", ids)
	}
}

func TestThemeResolveFillsTheRest(t *testing.T) {
	th, err := ThemeSpec{Name: "Mine", Background: "#000", Foreground: "#ffffff"}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if th.Background != "#000000" || th.SelectionText != "#ffffff" || th.MatchText != "#000000" || th.Gutter != th.CurrentLine {
		t.Errorf("resolved %+v", th)
	}
	for _, c := range colours(th) {
		if c == "" {
			t.Errorf("an empty colour in %+v", th)
		}
	}
	if _, err := (ThemeSpec{Name: "Bad", Background: "white", Foreground: "#000"}).Resolve(); err == nil || !strings.Contains(err.Error(), "background") {
		t.Errorf("a bad colour: %v", err)
	}
}

func TestThemesInTheFile(t *testing.T) {
	// The file's themes replace the built-in ones.
	c, err := Load(write(t, `{"editor_theme": "mine", "themes": {"mine": {"name": "Mine", "background": "#101010", "foreground": "#e0e0e0"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Themes) != 1 || c.Theme().Background != "#101010" {
		t.Errorf("themes %v, theme %+v", c.Themes, c.Theme())
	}
	// A theme that isn't there gives one that is, and is reported.
	c, err = Load(write(t, `{"editor_theme": "gone"}`))
	if err == nil || c.EditorTheme != DefaultTheme {
		t.Errorf("editor_theme %q, err %v", c.EditorTheme, err)
	}
	// A bad colour is reported; the default theme is used meanwhile.
	c, err = Load(write(t, `{"editor_theme": "bad", "themes": {"bad": {"name": "Bad", "background": "nope", "foreground": "#000"}}}`))
	if err == nil || !strings.Contains(err.Error(), "nope") || c.Theme().Name != BuiltinThemes[DefaultTheme].Name {
		t.Errorf("err %v, theme %q", err, c.Theme().Name)
	}
	// None at all gives the built-in ones.
	c, _ = Load(write(t, `{"themes": {}}`))
	if len(c.Themes) != len(BuiltinThemes) {
		t.Errorf("an empty list gave %v", c.Themes)
	}
	if NewThemeID(c.Themes, "Solarized Light") != "solarized-light-2" || NewThemeID(nil, "!!") != "theme" {
		t.Error("NewThemeID")
	}
}

func TestDirHonoursXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/x/y")
	if Dir() != "/x/y/notepad" {
		t.Errorf("Dir = %s", Dir())
	}
	t.Setenv("XDG_CONFIG_HOME", "relative")
	if !strings.HasSuffix(Dir(), "/.config/notepad") {
		t.Errorf("Dir with relative XDG = %s", Dir())
	}
}

func TestRecentFiles(t *testing.T) {
	list := AddRecent(nil, "/a", 3)
	list = AddRecent(list, "/b", 3)
	list = AddRecent(list, "/c", 3)
	list = AddRecent(list, "/a", 3)
	if strings.Join(list, " ") != "/a /c /b" {
		t.Errorf("after re-adding /a: %v", list)
	}
	list = AddRecent(list, "/d", 3)
	if strings.Join(list, " ") != "/d /a /c" {
		t.Errorf("beyond the limit: %v", list)
	}
	if got := RemoveRecent(list, "/a"); strings.Join(got, " ") != "/d /c" || len(list) != 3 {
		t.Errorf("RemoveRecent = %v, list %v", got, list)
	}
	if got := AddRecent(list, "/e", 0); len(got) != 0 {
		t.Errorf("with none remembered: %v", got)
	}
	c, err := Load(write(t, `{"recent_files_max": 2, "recent_files": ["/x", "", "/x", "/y", "/z"]}`))
	if err != nil || strings.Join(c.RecentFiles, " ") != "/x /y" {
		t.Errorf("loaded %v, %v", c.RecentFiles, err)
	}
	c, _ = Load(write(t, `{"recent_files_max": 500}`))
	if c.RecentFilesMax != MaxRecentFiles {
		t.Errorf("recent_files_max = %d", c.RecentFilesMax)
	}
}

// TestSetValuesOnNullFile checks that a settings file holding null, which
// Load takes as no settings, is written to rather than crashing.
func TestSetValuesOnNullFile(t *testing.T) {
	path := write(t, "null\n")
	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SetValues(path, map[string]any{"tab_width": 8}); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil || c.TabWidth != 8 {
		t.Errorf("tab_width %d, err %v", c.TabWidth, err)
	}
}
