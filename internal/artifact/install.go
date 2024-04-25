package artifact

import (
	"io"
	"io/fs"
	"os"
	"path"
)

// DefaultDirPerms are the permissions assigned to a directory when an Install* func is called
// and it has to create the parent directories for the destination.
const DefaultDirPerms = fs.ModeDir | 0755

// InstallFile installs src to dst with perms permissions. It ensures any base paths exist
// before installing.
func InstallFile(dst string, src io.Reader, perms fs.FileMode) error {
	if err := os.MkdirAll(path.Dir(dst), DefaultDirPerms); err != nil {
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
