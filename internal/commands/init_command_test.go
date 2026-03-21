package commands

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"strings"

	"github.com/aaron-vaz/skelly/internal/templates"
)

type mockDownloader struct {
	err error
}

func (m *mockDownloader) Get(ctx context.Context, source string, destination string) error {
	if m.err != nil {
		return m.err
	}
	// Create a dummy config in destination
	_ = os.MkdirAll(destination, 0755)
	configYaml := `
name: "test-template"
description: "A test template"
`
	_ = os.WriteFile(filepath.Join(destination, templates.TemplateConfigName), []byte(configYaml), 0644)
	return nil
}

type mockUIReturns struct {
	questionAnswer string
	questionErr    error
}

type mockUIInit struct {
	returns mockUIReturns
	infoMsgs []string
}

func (m *mockUIInit) RenderInputs(inputs map[string]templates.Input) error { return nil }
func (m *mockUIInit) RenderQuestion(question string, options []string) (string, error) {
	return m.returns.questionAnswer, m.returns.questionErr
}
func (m *mockUIInit) RenderInfo(message string) error {
	m.infoMsgs = append(m.infoMsgs, message)
	return nil
}
func (m *mockUIInit) RenderError(message string) error { return nil }

func TestInitCommand_Run(t *testing.T) {
	proc := templates.NewTemplateProcessor(templates.NewRendererService())

	tests := []struct {
		name         string
		setupCmd     func(tempDir string) *InitCommand
		args         []string
		preSetupDest bool
		wantErr      bool
		errMsg       string
		validateUI   func(t *testing.T, ui *mockUIInit)
	}{
		{
			name: "missing src option",
			setupCmd: func(tempDir string) *InitCommand {
				cmd := NewInitCommand(proc, &mockDownloader{}, &mockUIInit{})
				// Do not set --src
				return cmd.(*InitCommand)
			},
			args:    []string{},
			wantErr: true,
			errMsg:  "invalid options: source is required",
		},
		{
			name: "downloader error",
			setupCmd: func(tempDir string) *InitCommand {
				ui := &mockUIInit{}
				cmd := NewInitCommand(proc, &mockDownloader{err: errors.New("download failed")}, ui)
				flags := flag.NewFlagSet("init", flag.ContinueOnError)
				cmd.Init(flags)
				_ = flags.Parse([]string{"-src", "http://example.com/template", "-dst", filepath.Join(tempDir, "dest")})
				return cmd.(*InitCommand)
			},
			args:    []string{},
			wantErr: true,
			errMsg:  "failed to download template: download failed",
		},
		{
			name: "destination exists, user aborts",
			setupCmd: func(tempDir string) *InitCommand {
				ui := &mockUIInit{
					returns: mockUIReturns{questionAnswer: "n"},
				}
				cmd := NewInitCommand(proc, &mockDownloader{}, ui)
				flags := flag.NewFlagSet("init", flag.ContinueOnError)
				cmd.Init(flags)
				_ = flags.Parse([]string{"-src", "http://example.com/template", "-dst", filepath.Join(tempDir, "dest")})
				return cmd.(*InitCommand)
			},
			preSetupDest: true,
			args:         []string{},
			wantErr:      false,
			validateUI: func(t *testing.T, ui *mockUIInit) {
				if len(ui.infoMsgs) == 0 || !strings.Contains(ui.infoMsgs[0], "Exiting....") {
					t.Errorf("Expected exiting message")
				}
			},
		},
		{
			name: "successful init",
			setupCmd: func(tempDir string) *InitCommand {
				ui := &mockUIInit{}
				cmd := NewInitCommand(proc, &mockDownloader{}, ui)
				flags := flag.NewFlagSet("init", flag.ContinueOnError)
				cmd.Init(flags)
				_ = flags.Parse([]string{"-src", "http://example.com/template", "-dst", filepath.Join(tempDir, "dest")})
				return cmd.(*InitCommand)
			},
			args:    []string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			tempDir := t.TempDir()
			if tt.preSetupDest {
				_ = os.MkdirAll(filepath.Join(tempDir, "dest"), 0755)
			}

			cmd := tt.setupCmd(tempDir)
			
			// When
			err := cmd.Run(tt.args)

			// Then
			if tt.wantErr {
				if err == nil {
					t.Errorf("Run() error = nil, wantErr %v", tt.wantErr)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Run() error = %v, wantErrMsg contains %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				}
			}

			// And
			if tt.validateUI != nil {
				ui := cmd.ui.(*mockUIInit)
				tt.validateUI(t, ui)
			}
		})
	}
}
