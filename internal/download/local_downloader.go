package download

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// LocalDownloader implements Downloader for local directory copying.
// It bypasses remote fetching logic to perform direct, physical
// filesystem traversals and copies, avoiding symbolic linking issues.
type LocalDownloader struct{}

// Get executes a recursive directory copy from the specified source
// to the designated destination, preserving the local file hierarchy.
func (d *LocalDownloader) Get(ctx context.Context, source string, destination string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(destination, rel)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		srcF, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcF.Close()

		dstF, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstF.Close()

		_, err = io.Copy(dstF, srcF)
		return err
	})
}

// NewLocalDownloader initializes and returns a robust Downloader explicitly
// designed to safely traverse and copy local template directories.
func NewLocalDownloader() Downloader {
	return &LocalDownloader{}
}
