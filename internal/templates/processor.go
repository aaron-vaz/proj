package templates

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

var (
	pathSeparator = string(os.PathSeparator)
	dirsToSkip    = []string{".git"}
)

// Processor oversees the orchestration of template parsing, extracting configs,
// and invoking renderers against template directories.
type Processor struct {
	renderer *RendererService
}

// CreateTemplate reads the local configuration file located in the fetched template
// destination, unmarshaling it into a ProjectTemplate struct.
func (p *Processor) CreateTemplate(destination string) (*ProjectTemplate, error) {
	configBytes, err := os.ReadFile(filepath.Join(destination, TemplateConfigName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var config ProjectTemplate
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ApplyTemplate traverses the downloaded template project, executing two phases:
// executing file contents as text templates, and renaming files dynamically.
func (p *Processor) ApplyTemplate(config ProjectTemplate, destination string) error {
	// Skip certain paths
	// easier to do this by deleting them before processing
	for _, skip := range dirsToSkip {
		skipPath := filepath.Join(destination, skip)
		if _, err := os.Stat(skipPath); err == nil {
			if err := os.RemoveAll(skipPath); err != nil {
				return fmt.Errorf("error removing path %s: %w", skipPath, err)
			}
		}
	}

	// Phase 1: Render file contents in-place
	err := filepath.WalkDir(destination, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories (and symlinks to directories), That will be handled in phase 2
		stat, statErr := os.Stat(path)
		if statErr == nil && stat.IsDir() {
			return nil
		}

		// Skip the template config file itself
		if filepath.Base(path) == TemplateConfigName {
			return nil
		}

		return p.renderer.RenderFileContents(config, path)
	})

	if err != nil {
		return fmt.Errorf("error rendering file contents: %w", err)
	}

	// Phase 2: Rename files and directories from the bottom up
	pathsToRename := make([]string, 0)
	err = filepath.WalkDir(destination, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		pathsToRename = append(pathsToRename, path)
		return nil
	})

	if err != nil {
		return err
	}

	// Sort paths by depth, deepest first, to rename files before their parent directories
	sort.Slice(pathsToRename, func(i, j int) bool {
		return strings.Count(pathsToRename[i], pathSeparator) > strings.Count(pathsToRename[j], pathSeparator)
	})

	for _, path := range pathsToRename {
		// Skip the template config file itself
		if filepath.Base(path) == TemplateConfigName {
			continue
		}
		if _, err := p.renderer.RenderFile(config, path); err != nil {
			// It's possible the file was already moved as part of a parent directory rename, so ignore "not exist" errors
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("error renaming path %s: %w", path, err)
			}
		}
	}

	// Finally, delete the template config file
	return os.Remove(filepath.Join(destination, TemplateConfigName))
}

// NewTemplateProcessor instantiates a Processor equipped with a RendererService.
func NewTemplateProcessor(renderer *RendererService) *Processor {
	return &Processor{
		renderer: renderer,
	}
}
