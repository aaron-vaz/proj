package download

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGoGetterDownloader_Get(t *testing.T) {
	tests := []struct {
		name        string
		setupSource func(t *testing.T) string
		wantErr     bool
	}{
		{
			name: "successful local copy",
			setupSource: func(t *testing.T) string {
				srcDir := t.TempDir()
				// Create a dummy file in the source directory
				err := os.WriteFile(filepath.Join(srcDir, "dummy.txt"), []byte("hello world"), 0644)
				if err != nil {
					t.Fatalf("failed to setup source: %v", err)
				}
				return srcDir
			},
			wantErr: false,
		},
		{
			name: "error on invalid source",
			setupSource: func(t *testing.T) string {
				// use a non-existent path
				return filepath.Join(t.TempDir(), "nonexistent")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			src := tt.setupSource(t)
			dest := filepath.Join(t.TempDir(), "dest")

			downloader := NewGoGetterDownloader()

			// When
			err := downloader.Get(context.Background(), src, dest)

			// Then
			if tt.wantErr {
				if err == nil {
					t.Errorf("Get() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				}

				// And
				content, err := os.ReadFile(filepath.Join(dest, "dummy.txt"))
				if err != nil {
					t.Errorf("Failed to read downloaded file: %v", err)
				} else if string(content) != "hello world" {
					t.Errorf("Downloaded content mismatch: got %v, want %v", string(content), "hello world")
				}
			}
		})
	}
}
