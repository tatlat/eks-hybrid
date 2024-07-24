package artifact

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
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

// InstallTarGz untars the src file into the dst directory and deletes the src tgz file
func InstallTarGz(dst string, src string) error {
	if err := os.MkdirAll(dst, DefaultDirPerms); err != nil {
		return err
	}

	tgzExtractCmd := exec.Command("tar", "xvf", src, "-C", dst)
	if err := tgzExtractCmd.Run(); err != nil {
		return fmt.Errorf("unable to untar: %v", err)
	}

	// Remove the tgz file
	if err := os.Remove(src); err != nil {
		return err
	}
	return nil
}

func InstallPackage(packageName, packageManager string) error {
	installCmd := exec.Command(packageManager, "install", packageName, "-y")
	out, err := installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("running install command using package manager: %s, error: %v", out, err)
	}
	return nil
}

func UninstallPackage(packageName, packageManager string) error {
	removeCmd := exec.Command(packageManager, "remove", packageName, "-y")
	out, err := removeCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("running uninstall command using package manager: %s, error: %v", out, err)
	}
	return nil
}
