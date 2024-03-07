package artifact

import (
	"io"
	"io/fs"
	"os"
	"path"
)

// InstallFile installs src to dst with perms permissions. It ensures any base paths exist
// before installing.
func InstallFile(dst string, src Source, perms fs.FileMode) error {
	if err := os.MkdirAll(path.Dir(dst), fs.ModeDir|0775); err != nil {
		return err
	}

	fh, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_RDWR, perms)
	if err != nil {
		return err
	}
	defer fh.Close()

	_, err = io.Copy(fh, src)
	return err
}
