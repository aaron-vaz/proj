package download

import (
	"context"
	"os"
)

// SmartDownloader acts as a single router, delegating requests
// to either a localized physical downloader or a remote networking downloader
// conditionally based on the nature of the target source.
type SmartDownloader struct {
	local  Downloader
	remote Downloader
}

// Get conditionally inspects the source string. If the source resolves to
// an existing local directory, it passes the delegation to the local handler.
// Otherwise, it falls back to the remote handler implementation.
func (d *SmartDownloader) Get(ctx context.Context, source string, destination string) error {
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		return d.local.Get(ctx, source, destination)
	}
	
	return d.remote.Get(ctx, source, destination)
}

// NewSmartDownloader constructs an intelligent routing Downloader holding
// isolated local and remote implementations to handle varying URL behaviors.
func NewSmartDownloader(local Downloader, remote Downloader) Downloader {
	return &SmartDownloader{
		local:  local,
		remote: remote,
	}
}
