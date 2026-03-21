package download

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalDownloader_Get(t *testing.T) {
	// Given
	d := NewLocalDownloader()
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "dest")

	_ = os.WriteFile(filepath.Join(src, "test.txt"), []byte("hello"), 0644)

	// When
	err := d.Get(context.Background(), src, dest)

	// Then
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// And
	content, err := os.ReadFile(filepath.Join(dest, "test.txt"))
	if err != nil || string(content) != "hello" {
		t.Errorf("Expected 'hello', got %s", content)
	}
}
