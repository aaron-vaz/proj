package templates

import (
	"os"
	"strings"
	"text/template"
)

// StdRenderer implements the generic text/template behavior renderer.
type StdRenderer struct{}

// Supports always returns true as this is the universally active default fallback.
func (r *StdRenderer) Supports(config ProjectTemplate) bool {
	return true
}

// RenderFile executes standard Go templating against a file's name for dynamic structural renaming.
func (r *StdRenderer) RenderFile(config ProjectTemplate, path string) (string, error) {
	tmpl, err := template.New(path).Parse(path)
	if err != nil {
		return "", err
	}

	rPath := new(strings.Builder)
	err = tmpl.Execute(rPath, config)
	if err != nil {
		return "", err
	}

	if rPath.String() == path {
		return path, nil
	}

	err = os.Rename(path, rPath.String())
	if err != nil {
		return "", err
	}

	return rPath.String(), nil
}

// RenderFileContents reads, applies Go templates across a file's entire body,
// and effectively rewrites it directly while preserving original file modes.
func (r *StdRenderer) RenderFileContents(config ProjectTemplate, path string) error {
	// Get file info for permissions
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Read the original content
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Create a new template and parse the content
	tmpl, err := template.New(path).Parse(string(content))
	if err != nil {
		return err
	}

	// Execute the template into a buffer
	var rendered strings.Builder
	if err := tmpl.Execute(&rendered, config); err != nil {
		return err
	}

	// Write the rendered content back to the file
	return os.WriteFile(path, []byte(rendered.String()), info.Mode())
}
