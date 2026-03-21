package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binName = "skelly_test_bin"

func TestMain(m *testing.M) {
	// Build the CLI binary
	build := exec.Command("go", "build", "-o", binName)
	if err := build.Run(); err != nil {
		os.Stderr.WriteString("Could not build binary: " + err.Error() + "\n")
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Clean up
	_ = os.Remove(binName)

	os.Exit(code)
}

func TestIntegration_Help(t *testing.T) {
	// Given
	cmd := exec.Command("./"+binName, "help")

	// When
	out, err := cmd.CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("Expected no error, got %v: %s", err, string(out))
	}

	// And
	if !strings.Contains(string(out), "USAGE:") {
		t.Errorf("Expected help output, got %s", string(out))
	}
}

func TestIntegration_Version(t *testing.T) {
	// Given
	cmd := exec.Command("./"+binName, "version")

	// When
	out, err := cmd.CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("Expected no error, got %v: %s", err, string(out))
	}

	// And
	if !strings.Contains(string(out), "skelly version dev") {
		t.Errorf("Expected version output, got %s", string(out))
	}
}

func TestIntegration_Init_Success(t *testing.T) {
	// Given
	srcDir, _ := filepath.EvalSymlinks(t.TempDir())
	tmpDestBase, _ := filepath.EvalSymlinks(t.TempDir())
	destDir := filepath.Join(tmpDestBase, "dest")

	// Setup template
	configYaml := `
name: "integration-test"
description: "A test template"
inputs:
  project_name:
    description: "The name of the project"
    default: "my-app"
  author:
    description: "Author's name"
`
	err := os.WriteFile(filepath.Join(srcDir, ".skelly.yml"), []byte(configYaml), 0644)
	if err != nil {
		t.Fatalf("Failed to create config yaml: %v", err)
	}

	// Setup template files
	err = os.WriteFile(filepath.Join(srcDir, "{{.Inputs.project_name.Value}}.txt"), []byte("Author: {{.Inputs.author.Value}}"), 0644)
	if err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	// Prepare execution
	cmd := exec.Command("./"+binName, "init", "-src", srcDir, "-dst", destDir)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}

	// Provide interactive stdin input asynchronously
	go func() {
		defer stdin.Close()
		// Author input (required as no default)
		_, _ = io.WriteString(stdin, "John Doe\n")
		// Project_name input (has default "my-app", we hit enter to accept)
		_, _ = io.WriteString(stdin, "\n")
	}()

	// When
	out, err := cmd.CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("Expected no error from init run, got %v: %s", err, string(out))
	}

	// And
	expectedFile := filepath.Join(destDir, "my-app.txt")
	content, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Errorf("Expected generated file %s to exist, but it doesn't: %v. Output: %s", expectedFile, err, string(out))
	}

	// And
	if string(content) != "Author: John Doe" {
		t.Errorf("Expected generated file content 'Author: John Doe', got %s", string(content))
	}
}

func TestIntegration_Init_MissingSrc(t *testing.T) {
	// Given
	cmd := exec.Command("./"+binName, "init") // no -src provided

	// When
	out, err := cmd.CombinedOutput()

	// Then
	if err == nil {
		t.Fatalf("Expected an error for missing src, got nil")
	}

	// And
	if !strings.Contains(string(out), "invalid options: source is required") {
		t.Errorf("Expected message 'source is required', got %s", string(out))
	}
}
