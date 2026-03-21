package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStdRenderer_Supports(t *testing.T) {
	// Given
	r := &StdRenderer{}
	
	// When/Then
	if !r.Supports(ProjectTemplate{}) {
		t.Errorf("Expected StdRenderer to support any config")
	}
}

func TestStdRenderer_RenderFile(t *testing.T) {
	r := &StdRenderer{}
	cfg := ProjectTemplate{Name: "Foo"}

	t.Run("valid template rename", func(t *testing.T) {
		// Given
		dest := t.TempDir()
		tmplPath := filepath.Join(dest, "{{.Name}}_test.txt")
		_ = os.WriteFile(tmplPath, []byte(""), 0644)

		// When
		newPath, err := r.RenderFile(cfg, tmplPath)
		
		// Then
		if err != nil {
			t.Fatalf("RenderFile failed: %v", err)
		}

		// And
		expectedPath := filepath.Join(dest, "Foo_test.txt")
		if newPath != expectedPath {
			t.Errorf("Expected renamed path %s, got %s", expectedPath, newPath)
		}

		// And
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("Renamed file does not exist")
		}
	})

	t.Run("invalid template syntax", func(t *testing.T) {
		// Given
		dest := t.TempDir()
		tmplPath := filepath.Join(dest, "{{.Name_test.txt") // unclosed brace
		_ = os.WriteFile(tmplPath, []byte(""), 0644)

		// When
		_, err := r.RenderFile(cfg, tmplPath)
		
		// Then
		if err == nil {
			t.Errorf("Expected error for invalid template")
		}
	})
}

func TestStdRenderer_RenderFileContents(t *testing.T) {
	r := &StdRenderer{}
	cfg := ProjectTemplate{Name: "Bar"}

	t.Run("valid content rendering", func(t *testing.T) {
		// Given
		dest := t.TempDir()
		filePath := filepath.Join(dest, "test.txt")
		_ = os.WriteFile(filePath, []byte("Name is {{.Name}}"), 0644)

		// When
		err := r.RenderFileContents(cfg, filePath)
		
		// Then
		if err != nil {
			t.Fatalf("RenderFileContents failed: %v", err)
		}

		// And
		content, _ := os.ReadFile(filePath)
		if string(content) != "Name is Bar" {
			t.Errorf("Expected rendered content 'Name is Bar', got '%s'", string(content))
		}
	})

	t.Run("invalid content rendering", func(t *testing.T) {
		// Given
		dest := t.TempDir()
		filePath := filepath.Join(dest, "test.txt")
		_ = os.WriteFile(filePath, []byte("Name is {{.NonExistentField}}"), 0644)

		// When
		// text/template might not error on missing fields unless strict mode, actually it does if field doesn't exist on struct.
		// Wait, ProjectTemplate has no NonExistentField, text/template will emit <no value> or error out.
		// Let's test with a syntax error instead to guarantee failure
		_ = os.WriteFile(filePath, []byte("Name is {{."), 0644)
		err := r.RenderFileContents(cfg, filePath)
		
		// Then
		if err == nil {
			t.Errorf("Expected error for invalid template syntax")
		}
	})
}
