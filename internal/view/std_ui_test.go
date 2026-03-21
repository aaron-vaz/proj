package view

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aaron-vaz/skelly/internal/templates"
)

func TestStdUI_RenderInfo(t *testing.T) {
	// Given
	r, w, _ := os.Pipe()
	ui := NewStdUI(os.Stdin, w, os.Stderr)

	// When
	err := ui.RenderInfo("hello info")

	// Then
	if err != nil {
		t.Fatalf("RenderInfo failed: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	// And
	if !strings.Contains(buf.String(), "hello info\n") {
		t.Errorf("Expected info message, got %s", buf.String())
	}
}

func TestStdUI_RenderError(t *testing.T) {
	// Given
	r, w, _ := os.Pipe()
	ui := NewStdUI(os.Stdin, os.Stdout, w)

	// When
	err := ui.RenderError("hello error")

	// Then
	if err != nil {
		t.Fatalf("RenderError failed: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	// And
	if !strings.Contains(buf.String(), "hello error\n") {
		t.Errorf("Expected error message, got %s", buf.String())
	}
}

func TestStdUI_RenderQuestion(t *testing.T) {
	// Given
	inR, inW, _ := os.Pipe()
	outR, outW, _ := os.Pipe()

	ui := NewStdUI(inR, outW, os.Stderr)

	go func() {
		_, _ = inW.WriteString("y\n")
		inW.Close()
	}()

	// When
	ans, err := ui.RenderQuestion("Are you sure?", []string{"y", "n"})
	outW.Close()

	// Then
	if err != nil {
		t.Fatalf("RenderQuestion failed: %v", err)
	}

	// And
	if ans != "y" {
		t.Errorf("Expected answer 'y', got '%s'", ans)
	}

	var outBuf bytes.Buffer
	_, _ = io.Copy(&outBuf, outR)

	// And
	if !strings.Contains(outBuf.String(), "Are you sure?") {
		t.Errorf("Expected question prompt, got %s", outBuf.String())
	}
}

func TestStdUI_RenderInputs(t *testing.T) {
	// Given
	inR, inW, _ := os.Pipe()
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	_ = outR

	ui := NewStdUI(inR, outW, errW)

	go func() {
		// opt_input is asked first (sorted by key): Provide empty value
		_, _ = inW.WriteString("\n")
		// req_input is asked second: Provide empty value (triggers error and re-prompt)
		_, _ = inW.WriteString("\n")
		// req_input asked third: Provide valid value
		_, _ = inW.WriteString("valid-val\n")
		inW.Close()
	}()

	inputs := map[string]templates.Input{
		"req_input": {
			Description: "A required input",
			Default:     nil,
		},
		"opt_input": {
			Description: "An optional input",
			Default:     "def-val",
		},
	}

	// When
	err := ui.RenderInputs(inputs)
	outW.Close()
	errW.Close()

	// Then
	if err != nil {
		t.Fatalf("RenderInputs failed: %v", err)
	}

	// And
	if inputs["req_input"].Value != "valid-val" {
		t.Errorf("Expected req_input to be 'valid-val', got %v", inputs["req_input"].Value)
	}

	// And
	if inputs["opt_input"].Value != "def-val" {
		t.Errorf("Expected opt_input to be 'def-val', got %v", inputs["opt_input"].Value)
	}

	var errBuf bytes.Buffer
	_, _ = io.Copy(&errBuf, errR)

	// And
	if !strings.Contains(errBuf.String(), "is required. Please provide a value") {
		t.Errorf("Expected required error missing, got '%s'", errBuf.String())
	}
}

func TestStdUI_RenderInputs_Empty(t *testing.T) {
	// Given
	ui := NewStdUI(os.Stdin, os.Stdout, os.Stderr)

	// When
	err := ui.RenderInputs(map[string]templates.Input{})

	// Then
	if err != nil {
		t.Errorf("Expected nil on empty inputs, got %v", err)
	}
}
