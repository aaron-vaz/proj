package download

import (
	"context"
	"path/filepath"
	"testing"
)

type mockDownloader struct {
	called bool
}

func (m *mockDownloader) Get(ctx context.Context, source string, destination string) error {
	m.called = true
	return nil
}

func TestSmartDownloader_Get_Local(t *testing.T) {
	// Given
	localMock := &mockDownloader{}
	remoteMock := &mockDownloader{}
	d := NewSmartDownloader(localMock, remoteMock)
	
	src := t.TempDir() // Local directory
	
	// When
	err := d.Get(context.Background(), src, "dest")
	
	// Then
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// And
	if !localMock.called {
		t.Errorf("Expected local mock to be called")
	}
	
	// And
	if remoteMock.called {
		t.Errorf("Expected remote mock NOT to be called")
	}
}

func TestSmartDownloader_Get_Remote(t *testing.T) {
	// Given
	localMock := &mockDownloader{}
	remoteMock := &mockDownloader{}
	d := NewSmartDownloader(localMock, remoteMock)
	
	src := filepath.Join(t.TempDir(), "nonexistent") // Not an existing local path
	
	// When
	err := d.Get(context.Background(), src, "dest")
	
	// Then
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// And
	if localMock.called {
		t.Errorf("Expected local mock NOT to be called")
	}
	
	// And
	if !remoteMock.called {
		t.Errorf("Expected remote mock to be called")
	}
}
