package artifact_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/aws/eks-hybrid/internal/artifact"
)

func TestInstallFile(t *testing.T) {
	srcData := []byte("hello, world!")
	tmp := t.TempDir()
	src := bytes.NewBuffer(srcData)
	dst := filepath.Join(tmp, "file")
	perms := fs.FileMode(0o644)

	err := artifact.InstallFile(dst, src, perms)
	if err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}

	if fi.Mode() != perms {
		t.Fatalf("expected file to have perms %v; found %v", perms, fi.Mode())
	}

	dstData, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}

	if string(srcData) != string(dstData) {
		t.Fatalf("data read doesn't match: %s", dstData)
	}
}

func TestInstallFile_FileExists(t *testing.T) {
	tmp := t.TempDir()
	src := bytes.NewBufferString("hello, world!")
	dst := filepath.Join(tmp, "file")
	perms := fs.FileMode(0o644)

	if err := os.WriteFile(dst, []byte("hello, world!"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := artifact.InstallFile(dst, src, perms)
	if err != nil {
		t.Fatal(err)
	}
}

func TestInstallFile_DirNotExists(t *testing.T) {
	tmp := t.TempDir()
	src := bytes.NewBufferString("hello, world!")
	dir := filepath.Join(tmp, "nonexistent")
	dst := filepath.Join(dir, "file")
	perms := fs.FileMode(0o644)

	err := artifact.InstallFile(dst, src, perms)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !info.IsDir() {
		t.Fatalf("%v is not a direcftory", dir)
	}

	if info.Mode() != artifact.DefaultDirPerms {
		t.Fatalf("Expected dir with %v permissions; received %v", artifact.DefaultDirPerms, info.Mode())
	}
}

func tarGzBytes(t *testing.T, files map[string]struct {
	content string
	mode    int64
},
) []byte {
	buf := new(bytes.Buffer)
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	addedDirs := map[string]struct{}{}
	for name, file := range files {
		dir := filepath.Dir(name)
		if dir != "." && dir != "/" {
			parts := bytes.Split([]byte(dir), []byte("/"))
			for i := 1; i <= len(parts); i++ {
				d := string(bytes.Join(parts[:i], []byte("/")))
				if _, ok := addedDirs[d]; !ok {
					err := tw.WriteHeader(&tar.Header{
						Name:     d + "/",
						Typeflag: tar.TypeDir,
						Mode:     0o755,
					})
					if err != nil {
						t.Fatalf("failed to add dir %s: %v", d, err)
					}
					addedDirs[d] = struct{}{}
				}
			}
		}
		hdr := &tar.Header{
			Name: name,
			Mode: file.mode,
			Size: int64(len(file.content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write header for %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(file.content)); err != nil {
			t.Fatalf("failed to write content for %s: %v", name, err)
		}
	}
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestInstallTarGz(t *testing.T) {
	testcases := []struct {
		name  string
		files map[string]struct {
			content string
			mode    int64
		}
		setup  func(g *GomegaWithT, dst string)
		verify func(g *GomegaWithT, dst string, files map[string]struct {
			content string
			mode    int64
		})
		wantErr string // optional expected error message
	}{
		{
			name: "root file only",
			files: map[string]struct {
				content string
				mode    int64
			}{
				"root.txt": {"root content", 0o644},
			},
		},
		{
			name: "file in subdir",
			files: map[string]struct {
				content string
				mode    int64
			}{
				"subdir/file.txt": {"subdir content", 0o644},
			},
		},
		{
			name: "nested subdir file",
			files: map[string]struct {
				content string
				mode    int64
			}{
				"a/b/c.txt": {"deep content", 0o644},
			},
		},
		{
			name: "multiple files and dirs",
			files: map[string]struct {
				content string
				mode    int64
			}{
				"root.txt":  {"root", 0o644},
				"d1/f1.txt": {"f1", 0o644},
				"d1/f2.txt": {"f2", 0o644},
				"d2/f3.txt": {"f3", 0o644},
			},
		},
		{
			name: "files with different permissions",
			files: map[string]struct {
				content string
				mode    int64
			}{
				"executable.sh": {"#!/bin/sh\necho hello", 0o755},
				"readonly.txt":  {"read only", 0o444},
				"private.txt":   {"private", 0o600},
			},
		},
		{
			name: "extract to pre-existing directories",
			files: map[string]struct {
				content string
				mode    int64
			}{
				"dir1/file1.txt":      {"content1", 0o644},
				"dir2/file2.txt":      {"content2", 0o644},
				"dir1/dir3/file3.txt": {"content3", 0o644},
			},
			setup: func(g *GomegaWithT, dst string) {
				// Create some directories with different permissions before extraction
				dirs := []string{"dir1", "dir2", "dir1/dir3"}
				for _, dir := range dirs {
					dirPath := filepath.Join(dst, dir)
					g.Expect(os.MkdirAll(dirPath, 0o700)).To(Succeed())
				}
			},
			verify: func(g *GomegaWithT, dst string, files map[string]struct {
				content string
				mode    int64
			},
			) {
				// Verify that pre-existing directories maintain their permissions
				preExistingDirs := []string{"dir1", "dir2", "dir1/dir3"}
				for _, dir := range preExistingDirs {
					dirPath := filepath.Join(dst, dir)
					info, err := os.Stat(dirPath)
					g.Expect(err).NotTo(HaveOccurred(), "failed to stat pre-existing dir %s", dir)
					g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o700)), "pre-existing dir %s permissions were changed", dir)
				}
			},
		},
		{
			name: "invalid relative paths",
			files: map[string]struct {
				content string
				mode    int64
			}{
				"../outside.txt":                 {"should not escape", 0o644},
				"dir1/../../outside.txt":         {"should not escape", 0o644},
				"dir1/../dir2/../../outside.txt": {"should not escape", 0o644},
				"normal.txt":                     {"normal content", 0o644},
			},
			wantErr: "tar contained invalid name error",
		},
		{
			name: "relative path traversal attempt",
			files: map[string]struct {
				content string
				mode    int64
			}{
				"normal.txt": {"normal content", 0o644},
			},
			setup: func(g *GomegaWithT, dst string) {
				// Create a file outside the destination to verify it's not overwritten
				outsidePath := filepath.Join(filepath.Dir(dst), "outside.txt")
				g.Expect(os.WriteFile(outsidePath, []byte("original content"), 0o644)).To(Succeed())
			},
			verify: func(g *GomegaWithT, dst string, files map[string]struct {
				content string
				mode    int64
			},
			) {
				// Verify the original outside file wasn't modified
				outsidePath := filepath.Join(filepath.Dir(dst), "outside.txt")
				content, err := os.ReadFile(outsidePath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(content)).To(Equal("original content"), "outside file was modified")
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			tmp := t.TempDir()
			src := filepath.Join(tmp, "test.tar.gz")
			g.Expect(os.WriteFile(src, tarGzBytes(t, tc.files), 0o644)).To(Succeed())
			dst := filepath.Join(tmp, "dst")

			// Run setup if provided
			if tc.setup != nil {
				tc.setup(g, dst)
			}

			err := artifact.InstallTarGz(dst, src)
			if tc.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(ContainSubstring(tc.wantErr)))
			}
			// Run verification if provided
			if tc.verify != nil {
				tc.verify(g, dst, tc.files)
			}

			if tc.wantErr != "" {
				return
			}

			g.Expect(err).NotTo(HaveOccurred())

			// verify source file is removed
			g.Expect(os.Stat(src)).Error().To(MatchError(ContainSubstring("no such file or directory")), "source file not removed after extraction")

			// Generic verification: check all files and their parent directories
			for p, file := range tc.files {
				fullPath := filepath.Join(dst, p)
				content, err := os.ReadFile(fullPath)
				g.Expect(err).NotTo(HaveOccurred(), "%s not extracted", p)
				g.Expect(string(content)).To(Equal(file.content), "%s content mismatch", p)

				// Verify file permissions
				info, err := os.Stat(fullPath)
				g.Expect(err).NotTo(HaveOccurred(), "failed to stat %s", p)
				g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(file.mode&0o777)), "%s has wrong permissions", p)

				// Check all parent directories exist
				dir := filepath.Dir(fullPath)
				for dir != dst && dir != "." && dir != "/" {
					info, err := os.Stat(dir)
					g.Expect(err).NotTo(HaveOccurred(), "parent dir %s not created", dir)
					g.Expect(info.IsDir()).To(BeTrue(), "parent dir %s is not a directory", dir)
					dir = filepath.Dir(dir)
				}
			}
		})
	}
}
