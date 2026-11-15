package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEntry(t *testing.T) {
	entry := Entry("/home/me/bin/notepad", "notepad", "/home/me/.local/share/icons/hicolor/256x256/apps/notepad.png")
	for _, line := range []string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=Notepad",
		"Exec=/home/me/bin/notepad %F",
		"MimeType=text/plain;",
		"TryExec=/home/me/bin/notepad",
		"Icon=/home/me/.local/share/icons/hicolor/256x256/apps/notepad.png",
		"StartupWMClass=notepad",
	} {
		if !strings.Contains(entry, line+"\n") {
			t.Errorf("no %q in\n%s", line, entry)
		}
	}
	if !strings.HasPrefix(entry, "[Desktop Entry]\n") {
		t.Errorf("entry does not start with its group:\n%s", entry)
	}
}

func TestExecQuoting(t *testing.T) {
	for path, want := range map[string]string{
		"/opt/notepad":                      "/opt/notepad",
		"/home/me/My Apps/notepad.AppImage": `"/home/me/My Apps/notepad.AppImage"`,
		`/tmp/a"b`:                          `"/tmp/a\\"b"`,
		"/tmp/$HOME/x":                      `"/tmp/\\$HOME/x"`,
		"/tmp/100%/x":                       "/tmp/100%%/x",
		`/tmp/back\slash`:                   `"/tmp/back\\\\slash"`,
	} {
		entry := Entry(path, "x", "/x.png")
		if !strings.Contains(entry, "\nExec="+want+" %F\n") {
			t.Errorf("Exec for %q: want %q in\n%s", path, want, entry)
		}
	}
}

func TestExecutable(t *testing.T) {
	// An AppImage is started again through the AppImage itself.
	t.Setenv("APPIMAGE", "/home/me/Apps/notepad-x86_64.AppImage")
	if got, err := Executable(); err != nil || got != "/home/me/Apps/notepad-x86_64.AppImage" {
		t.Errorf("in an AppImage: %q, %v", got, err)
	}

	// Otherwise it is the program itself, with any links followed.
	t.Setenv("APPIMAGE", "")
	got, err := Executable()
	if err != nil || !filepath.IsAbs(got) {
		t.Fatalf("Executable() = %q, %v", got, err)
	}
	if resolved, _ := filepath.EvalSymlinks(got); resolved != got {
		t.Errorf("links not followed: %q is %q", got, resolved)
	}
}

func TestDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/data/home")
	if got, err := Dir(); err != nil || got != "/data/home" {
		t.Errorf("with XDG_DATA_HOME: %q, %v", got, err)
	}
	// A relative one is ignored, as the specification says.
	t.Setenv("XDG_DATA_HOME", "relative")
	t.Setenv("HOME", "/home/me")
	if got, err := Dir(); err != nil || got != "/home/me/.local/share" {
		t.Errorf("without XDG_DATA_HOME: %q, %v", got, err)
	}
}

func TestWrite(t *testing.T) {
	data := t.TempDir()
	entryPath := filepath.Join(data, "applications", "notepad.desktop")
	iconPath := filepath.Join(data, "icons", "hicolor", "256x256", "apps", "notepad.png")

	if err := Write(data, "/opt/one/notepad", "notepad", []byte("png")); err != nil {
		t.Fatal(err)
	}
	entry, err := os.ReadFile(entryPath)
	if err != nil || !strings.Contains(string(entry), "Exec=/opt/one/notepad %F\n") {
		t.Fatalf("entry %q, %v", entry, err)
	}
	if icon, err := os.ReadFile(iconPath); err != nil || string(icon) != "png" {
		t.Errorf("icon %q, %v", icon, err)
	}
	if info, err := os.Stat(entryPath); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("entry mode %v, %v", info.Mode(), err)
	}

	// The same again leaves the files alone.
	old := time.Now().Add(-time.Hour)
	os.Chtimes(entryPath, old, old)
	if err := Write(data, "/opt/one/notepad", "notepad", []byte("png")); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(entryPath); !info.ModTime().Equal(old) {
		t.Error("an entry that was already right was written again")
	}

	// Run from elsewhere, or under another name, it is rewritten.
	if err := Write(data, "/home/me/Apps/notepad.AppImage", "notepad", []byte("png")); err != nil {
		t.Fatal(err)
	}
	entry, _ = os.ReadFile(entryPath)
	if !strings.Contains(string(entry), "Exec=/home/me/Apps/notepad.AppImage %F\n") || strings.Contains(string(entry), "/opt/one") {
		t.Errorf("entry not rewritten:\n%s", entry)
	}
	// No temporary files are left behind.
	if files, _ := os.ReadDir(filepath.Dir(entryPath)); len(files) != 1 {
		t.Errorf("files left in applications: %v", files)
	}
}

func TestInstall(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("APPIMAGE", "/home/me/Apps/My notepad.AppImage")

	icon := []byte("\x89PNG icon")
	if err := Install(icon); err != nil {
		t.Fatal(err)
	}

	// The entry starts the AppImage, quoted as its name has a space.
	entry, err := os.ReadFile(filepath.Join(data, "applications", "notepad.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	self, _ := os.Executable()
	// The icon is named by its path, not looked for in the theme.
	image := filepath.Join(data, "icons", "hicolor", "256x256", "apps", "notepad.png")
	for _, line := range []string{`Exec="/home/me/Apps/My notepad.AppImage" %F`, "Icon=" + image, "StartupWMClass=" + filepath.Base(self)} {
		if !strings.Contains(string(entry), line+"\n") {
			t.Errorf("no %q in\n%s", line, entry)
		}
	}

	// The icon is the one given.
	if got, err := os.ReadFile(image); err != nil || string(got) != string(icon) {
		t.Errorf("icon %q, %v", got, err)
	}
}

// TestInstallSkipsTemporaryProgram checks that a program run from the
// temporary folder, as go run builds one, leaves the desktop entry alone.
func TestInstallSkipsTemporaryProgram(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("APPIMAGE", filepath.Join(os.TempDir(), "go-build123", "b001", "exe", "notepad"))
	if err := Install([]byte("png")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, "applications", "notepad.desktop")); !os.IsNotExist(err) {
		t.Errorf("an entry was written: %v", err)
	}
	for path, want := range map[string]bool{
		filepath.Join(os.TempDir(), "x"): true,
		os.TempDir() + "x/y":             false,
		"/home/me/bin/notepad":           false,
	} {
		if got := temporary(path); got != want {
			t.Errorf("temporary(%q) = %v", path, got)
		}
	}
}
