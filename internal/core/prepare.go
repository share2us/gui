package core

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	clicore "github.com/share2us/cli-core"
)

// prepared is the result of normalising a path into something uploadable: a
// single file, with a directory transparently zipped first.
type prepared struct {
	path         string // file to read for upload (a temp zip for folders)
	name         string // display file name shown to the recipient
	size         int64
	contentType  string
	contentClass string
	isFolder     bool
	cleanup      func() // removes any temp zip; always safe to call
}

// prepareContent stats path and, when it is a directory, zips it to a temp file
// (folders can only be sent to a device/contact, matching the CLI). The caller
// must defer p.cleanup().
func prepareContent(path string) (prepared, error) {
	info, err := os.Stat(path)
	if err != nil {
		return prepared{cleanup: func() {}}, err
	}
	if info.IsDir() {
		zipped, err := zipDirectory(path)
		if err != nil {
			return prepared{cleanup: func() {}}, err
		}
		cleanup := func() { _ = os.Remove(zipped) }
		zi, err := os.Stat(zipped)
		if err != nil {
			cleanup()
			return prepared{cleanup: func() {}}, err
		}
		return prepared{
			path:         zipped,
			name:         directoryZipName(path),
			size:         zi.Size(),
			contentType:  "application/zip",
			contentClass: clicore.ContentClassFolder,
			isFolder:     true,
			cleanup:      cleanup,
		}, nil
	}
	name := filepath.Base(path)
	ct := contentTypeForName(name)
	return prepared{
		path:         path,
		name:         name,
		size:         info.Size(),
		contentType:  ct,
		contentClass: clicore.ContentClassForNameAndType(name, ct),
		cleanup:      func() {},
	}, nil
}

// contentTypeForName guesses a MIME type from the extension, defaulting to
// application/octet-stream.
func contentTypeForName(name string) string {
	if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// fileSHA256 returns the lowercase hex SHA-256 of a file.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// encryptToTemp stream-encrypts src with key into a temp file and returns its
// path and size. Caller removes the temp file.
func encryptToTemp(srcPath string, key []byte) (string, int64, error) {
	tmp, err := os.CreateTemp("", "share2us-enc-*")
	if err != nil {
		return "", 0, err
	}
	tmpPath := tmp.Name()
	src, err := os.Open(srcPath)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", 0, err
	}
	if err := clicore.EncryptStream(tmp, src, key); err != nil {
		_ = src.Close()
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", 0, err
	}
	_ = src.Close()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", 0, err
	}
	info, err := os.Stat(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", 0, err
	}
	return tmpPath, info.Size(), nil
}

// directoryZipName renders "<folder>.zip" for a zipped directory upload.
// Ported verbatim from the CLI so folder names match.
func directoryZipName(path string) string {
	name := filepath.Base(filepath.Clean(path))
	if name == "" || name == "." || name == string(os.PathSeparator) {
		return "folder.zip"
	}
	return name + ".zip"
}

// zipDirectory zips root into a temp .zip and returns its path. Ported from the
// CLI (main.go zipDirectory) including symlink handling, so a folder shared from
// the app is byte-identical to one shared from the CLI.
func zipDirectory(root string) (string, error) {
	root = filepath.Clean(root)
	tmp, err := os.CreateTemp("", "share2us-folder-*.zip")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	zw := zip.NewWriter(tmp)
	fail := func(e error) (string, error) {
		_ = zw.Close()
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", e
	}

	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if entry.IsDir() {
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = strings.TrimRight(name, "/") + "/"
			_, err = zw.CreateHeader(header)
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Deflate
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			writer, err := zw.CreateHeader(header)
			if err != nil {
				return err
			}
			_, err = writer.Write([]byte(target))
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(writer, src)
		return err
	})
	if walkErr != nil {
		return fail(walkErr)
	}
	if err := zw.Close(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

// uniquePath returns p if free, else "p (1)", "p (2)", ... so a receive never
// overwrites an existing file in Downloads.
func uniquePath(p string) string {
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return p
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}
