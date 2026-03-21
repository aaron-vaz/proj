// Package download provides implementations for downloading and copying
// templates from local or remote sources.
package download

import (
	"context"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-getter/v2"
)

// GoGetterDownloader is an implementation of the Downloader interface
// that strictly relies on the hashicorp/go-getter library to fetch
// configurations and templates globally.
type GoGetterDownloader struct{}

// Get delegates the fetching process to the underlying go-getter client,
// converting local paths to absolute paths for compatibility.
func (d *GoGetterDownloader) Get(ctx context.Context, source string, destination string) error {
	// For local sources, go-getter expects an absolute path.
	// We check if the source is a local path, and if so, convert it to an absolute path.
	if _, err := os.Stat(source); err == nil {
		absSrc, err := filepath.Abs(source)
		if err != nil {
			return err // Could not get absolute path
		}
		source = absSrc
	}

	_, err := getter.GetAny(ctx, destination, source)
	return err
}

// NewGoGetterDownloader creates and returns a new Downloader configured
// to fetch remote artifacts using hashicorp/go-getter.
func NewGoGetterDownloader() Downloader {
	return &GoGetterDownloader{}
}
