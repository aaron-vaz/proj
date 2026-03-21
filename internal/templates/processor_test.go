package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProcessor_CreateTemplate(t *testing.T) {
	proc := NewTemplateProcessor(NewRendererService())

	tests := []struct {
		name         string
		setupDest    func(t *testing.T, dest string)
		wantNil      bool
		wantErr      bool
		wantName     string
	}{
		{
			name: "no config file",
			setupDest: func(t *testing.T, dest string) {
				// empty dir
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "valid config file",
			setupDest: func(t *testing.T, dest string) {
				config := `name: test-project`
				os.WriteFile(filepath.Join(dest, TemplateConfigName), []byte(config), 0644)
			},
			wantNil:  false,
			wantErr:  false,
			wantName: "test-project",
		},
		{
			name: "invalid config file",
			setupDest: func(t *testing.T, dest string) {
				config := `name: [invalid yaml`
				os.WriteFile(filepath.Join(dest, TemplateConfigName), []byte(config), 0644)
			},
			wantNil: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			dest := t.TempDir()
			tt.setupDest(t, dest)

			// When
			cfg, err := proc.CreateTemplate(dest)

			// Then
			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateTemplate() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("CreateTemplate() error = %v, wantErr %v", err, tt.wantErr)
				}
				
				// And
				if tt.wantNil && cfg != nil {
					t.Errorf("Expected nil config but got %v", cfg)
				}
				if !tt.wantNil {
					if cfg == nil {
						t.Errorf("Expected non-nil config")
					} else if cfg.Name != tt.wantName {
						t.Errorf("Expected Name = %v, got %v", tt.wantName, cfg.Name)
					}
				}
			}
		})
	}
}

func TestProcessor_ApplyTemplate(t *testing.T) {
	proc := NewTemplateProcessor(NewRendererService())

	t.Run("successful apply", func(t *testing.T) {
		// Given
		dest := t.TempDir()

		// Setup .skelly.yaml
		os.WriteFile(filepath.Join(dest, TemplateConfigName), []byte(""), 0644)

		// Setup .git dir to be skipped
		gitDir := filepath.Join(dest, ".git")
		os.MkdirAll(gitDir, 0755)
		os.WriteFile(filepath.Join(gitDir, "ignore.txt"), []byte("ignored"), 0644)

		// Setup a template file
		os.WriteFile(filepath.Join(dest, "{{.Name}}.txt"), []byte("Hello {{.Name}}!"), 0644)

		// Create a config
		cfg := ProjectTemplate{
			Name: "World",
		}

		// When
		err := proc.ApplyTemplate(cfg, dest)
		
		// Then
		if err != nil {
			t.Fatalf("ApplyTemplate failed: %v", err)
		}

		// And
		if _, err := os.Stat(gitDir); !os.IsNotExist(err) {
			t.Errorf("Expected .git to be removed")
		}

		// And
		if _, err := os.Stat(filepath.Join(dest, TemplateConfigName)); !os.IsNotExist(err) {
			t.Errorf("Expected %s to be removed", TemplateConfigName)
		}

		// And
		renamedFile := filepath.Join(dest, "World.txt")
		content, err := os.ReadFile(renamedFile)
		if err != nil {
			t.Errorf("Expected renamed file World.txt to exist: %v", err)
		} else if string(content) != "Hello World!" {
			t.Errorf("Expected file content 'Hello World!', got '%s'", string(content))
		}
	})
}
