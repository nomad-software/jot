// Package desktop writes the desktop entry which lets a Linux desktop list
// Notepad with its other applications, to be started from there and kept
// in a dock or sidebar. It has no dependency on the UI.
package desktop

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"notepad/internal/atomicfile"
)

// Name is the name of the desktop entry, without its extension, and of the
// icon. It is the name main.go gives Qt (QGuiApplication_SetDesktopFileName),
// by which a Wayland desktop matches a window to its entry.
const Name = "notepad"

// DisplayName is the name people see: in menus, window titles and dialogs.
const DisplayName = "Notepad"

// Executable returns the file that starts the program as it is being run: the
// AppImage it is in, if it is in one, and otherwise the program itself. Inside
// an AppImage the program is in a folder that is only there while it runs, so
// the AppImage, which its runtime names in $APPIMAGE, is what starts it again.
func Executable() (string, error) {
	if image := os.Getenv("APPIMAGE"); image != "" {
		return image, nil
	}
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path, nil
}

// Dir returns the user's data folder, $XDG_DATA_HOME or ~/.local/share
// without it, which holds their desktop entries (in applications) and
// icons.
func Dir() (string, error) {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" || !filepath.IsAbs(data) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		data = filepath.Join(home, ".local", "share")
	}
	return data, nil
}

// Entry returns the desktop entry for a program started by the file at exec.
// The class is what the program's windows are known by under X11, the name
// of the program file, so that its windows are matched to the entry. The
// icon is the path of its image file: a path and not the name of an icon of
// the theme, which a desktop may not find in a folder it has a stale cache
// of (as GNOME on Ubuntu 22.04 does), leaving the windows matched to the
// entry with no icon at all.
func Entry(exec, class, icon string) string {
	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Type=Application\n")
	b.WriteString("Name=" + DisplayName + "\n")
	b.WriteString("GenericName=Text Editor\n")
	b.WriteString("Comment=Simple text editor with tabs\n")
	// %F: the files chosen in a file manager are passed to it to open.
	b.WriteString("Exec=" + escape(quote(exec)) + " %F\n")
	b.WriteString("TryExec=" + escape(exec) + "\n")
	b.WriteString("Icon=" + escape(icon) + "\n")
	b.WriteString("Categories=Utility;TextEditor;\n")
	b.WriteString("Keywords=text;editor;plain;notepad;\n")
	b.WriteString("MimeType=text/plain;\n")
	b.WriteString("Terminal=false\n")
	b.WriteString("StartupWMClass=" + escape(class) + "\n")
	return b.String()
}

// quote makes a path one argument of an Exec key: in double quotes if it has
// anything the desktop entry specification reserves, with ", `, $ and \
// escaped inside them, and with % doubled, which would otherwise begin a
// field code.
func quote(path string) string {
	path = strings.ReplaceAll(path, "%", "%%")
	if !strings.ContainsAny(path, " \t\n\"'\\><~|&;$*?#()`") {
		return path
	}
	escaped := strings.NewReplacer(`"`, `\"`, "`", "\\`", `$`, `\$`, `\`, `\\`).Replace(path)
	return `"` + escaped + `"`
}

// escape escapes a value of a desktop entry key, in which a backslash and
// the characters it could stand for are written with one.
func escape(value string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\t", `\t`, "\r", `\r`).Replace(value)
}

// Install writes the desktop entry and its icon, as PNG data, that let a
// Linux desktop list Notepad with its applications, so that it can be
// started from there and kept in a dock or sidebar. It is called at every
// start, and rewrites the entry whenever the program is run from somewhere
// else or under another name, so that the entry always starts the copy last
// run: an AppImage, or else the program itself. A program in the
// temporary folder, as go run builds, is not one to start again, and the
// entry is left as it is.
func Install(icon []byte) error {
	exec, err := Executable()
	if err != nil {
		return err
	}
	if temporary(exec) {
		return nil
	}
	data, err := Dir()
	if err != nil {
		return err
	}
	// Windows are known under X11 by the name of the program file that is
	// running, which inside an AppImage is the program and not the image.
	class := Name
	if self, err := os.Executable(); err == nil {
		class = filepath.Base(self)
	}
	return Write(data, exec, class, icon)
}

// Write writes the desktop entry and the icon, as PNG data, into the data
// folder given: the entry to applications/notepad.desktop and the icon to
// icons/hicolor/256x256/apps/notepad.png. Each is written only if it is
// not already so, and in a way that leaves the old file whole if it fails.
func Write(data, exec, class string, icon []byte) error {
	entry := filepath.Join(data, "applications", Name+".desktop")
	image := filepath.Join(data, "icons", "hicolor", "256x256", "apps", Name+".png")
	return errors.Join(
		writeFile(image, icon),
		writeFile(entry, []byte(Entry(exec, class, image))),
	)
}

// writeFile writes data to a file through a temporary one beside it, unless
// it already holds that data.
func writeFile(path string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	return atomicfile.Write(path, data, 0o644)
}

// temporary reports whether path is in the temporary folder.
func temporary(path string) bool {
	rel, err := filepath.Rel(os.TempDir(), path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
