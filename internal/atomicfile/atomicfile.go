// Package atomicfile writes files so that a failure part way leaves the old
// file whole: through a temporary file beside it, renamed over it.
package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// Write writes data to path through a temporary file in the same folder,
// which is then renamed over it. The folder is made if it is not there,
// and the file is given mode.
func Write(path string, data []byte, mode os.FileMode) error {
	return write(path, data, mode, -1)
}

// write is Write, also giving the file the group gid unless it is -1.
func write(path string, data []byte, mode os.FileMode, gid int) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(data)
	if err == nil && gid >= 0 {
		err = tmp.Chown(-1, gid)
	}
	if err == nil {
		// After Chown, which clears the setuid and setgid bits.
		err = tmp.Chmod(mode)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	syncDir(dir)
	return nil
}

// syncDir makes a rename in dir last through a crash. It is done where it
// can be: some file systems cannot sync a folder, and the file is written
// whatever happens.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
}

// Save writes a document to path, as an editor saves one. A symbolic link
// has its target written, not replaced. An existing file keeps its
// permissions (setuid, setgid and sticky bits too) and group; a new one is
// made 0644 (less the umask). The file is replaced through a temporary file
// when that changes nothing else about it, and otherwise written in place:
// when it has other hard links, belongs to someone else, its group cannot
// be given to the new file, its folder cannot be written to, or it cannot
// be replaced (a file mounted on its own, or across file systems).
func Save(path string, data []byte) error {
	path = Resolve(path)
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return writeInPlace(path, data, 0o644)
	case err != nil:
		return err
	}
	mode := info.Mode() & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	if !info.Mode().IsRegular() {
		return writeInPlace(path, data, mode)
	}
	gid := -1
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		if st.Nlink > 1 || int(st.Uid) != os.Getuid() {
			return writeInPlace(path, data, mode)
		}
		gid = int(st.Gid)
	}
	if err := write(path, data, mode, gid); err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EBUSY) || errors.Is(err, syscall.EXDEV) {
			return writeInPlace(path, data, mode)
		}
		return err
	}
	return nil
}

// Resolve returns the file a path names, following symbolic links, a
// dangling one too.
func Resolve(path string) string {
	if target, err := filepath.EvalSymlinks(path); err == nil {
		return target
	}
	if link, err := os.Readlink(path); err == nil {
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(path), link)
		}
		return link
	}
	return path
}

// writeInPlace writes data over the file itself, which keeps everything
// about it but, if it fails part way, not its old contents. mode is a new
// file's.
func writeInPlace(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		// A pipe or device cannot be synced (EINVAL), which is no failure.
		if serr := f.Sync(); serr != nil && !errors.Is(serr, syscall.EINVAL) {
			err = serr
		}
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
