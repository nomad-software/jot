package atomicfile

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSaveNewAndExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.txt")
	if err := Save(path, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if read(t, path) != "one" {
		t.Fatal("not written")
	}
	os.Chmod(path, 0o600)
	if err := Save(path, []byte("two")); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); read(t, path) != "two" || info.Mode().Perm() != 0o600 {
		t.Errorf("content %q, mode %v", read(t, path), info.Mode().Perm())
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("files left behind: %v", entries)
	}
}

func TestSaveFollowsLinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")
	os.WriteFile(target, []byte("old"), 0o644)
	os.Symlink(target, link)
	if err := Save(link, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Error("the link was replaced by a file")
	}
	if read(t, target) != "new" {
		t.Error("the target was not written")
	}
	// A dangling link has its target made.
	dangling := filepath.Join(dir, "dangling.txt")
	os.Symlink("made.txt", dangling)
	if err := Save(dangling, []byte("made")); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "made.txt")) != "made" {
		t.Error("a dangling link's target was not made")
	}
}

func TestSaveKeepsHardLinks(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	os.WriteFile(a, []byte("old"), 0o644)
	if err := os.Link(a, b); err != nil {
		t.Skip(err)
	}
	if err := Save(a, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if read(t, b) != "new" {
		t.Error("the other hard link no longer shares the file")
	}
}

func TestSaveInReadOnlyFolder(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root writes anywhere")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "a")
	os.WriteFile(path, []byte("old"), 0o644)
	os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755)
	if err := Save(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if read(t, path) != "new" {
		t.Error("not written")
	}
}

// TestSaveKeepsGroupAndSpecialBits checks that a file replaced through a
// temporary one keeps its group and its setgid bit.
func TestSaveKeepsGroupAndSpecialBits(t *testing.T) {
	groups, err := os.Getgroups()
	if err != nil || len(groups) < 2 {
		t.Skip("needs a user in two groups")
	}
	path := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(path, []byte("old"), 0o664); err != nil {
		t.Fatal(err)
	}
	gid := groups[0]
	if gid == os.Getgid() {
		gid = groups[1]
	}
	if err := os.Chown(path, -1, gid); err != nil {
		t.Skip(err)
	}
	if err := os.Chmod(path, 0o664|os.ModeSetgid); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := int(info.Sys().(*syscall.Stat_t).Gid); got != gid {
		t.Errorf("group %d, want %d", got, gid)
	}
	if info.Mode()&os.ModeSetgid == 0 || info.Mode().Perm() != 0o664 {
		t.Errorf("mode %v, want setgid and 0664", info.Mode())
	}
}
