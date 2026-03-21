package view

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/aaron-vaz/skelly/internal/templates"
)

// StdUI is a standard I/O implementation of the UI interface, facilitating
// interaction with the user via os streams.
type StdUI struct {
	stdin  *bufio.Scanner
	stdout *os.File
	stderr *os.File
}

// RenderInputs interactively prompts the user for all parameters defined in the template config.
func (s *StdUI) RenderInputs(inputs map[string]templates.Input) error {
	// No need to render if there are no inputs defined in the config
	// this is not an error, just a no-op
	if len(inputs) == 0 {
		return nil
	}

	_, err := fmt.Fprintf(s.stdout, "Please provide values for inputs defined in %s:\n\n", templates.TemplateConfigName)
	if err != nil {
		return err
	}

	// Sort the keys to ensure a consistent order
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		input := inputs[name]
		err := s.renderInput(name, &input)
		if err != nil {
			return err
		}

		inputs[name] = input
	}

	return nil
}

// RenderQuestion poses a question to the user and awaits a formatted string response.
func (s *StdUI) RenderQuestion(question string, options []string) (string, error) {
	_, err := fmt.Fprintf(s.stdout, "%s? [%s]\n", question, strings.Join(options, "/"))
	if err != nil {
		return "", err
	}

	return s.waitForUserInput()
}

func (s *StdUI) RenderInfo(message string) error {
	_, err := fmt.Fprintf(s.stdout, "%s\n", message)
	return err
}

func (s *StdUI) RenderError(message string) error {
	_, err := fmt.Fprintf(s.stderr, "%s\n", message)
	return err
}

func (s *StdUI) renderInput(name string, input *templates.Input) error {
	required := false
	defaultValue := input.Default
	if defaultValue == "" || defaultValue == nil {
		required = true
		defaultValue = "*" // required
	}

	_, err := fmt.Fprintf(s.stdout, "%s: \n%s: [%s]\n", name, input.Description, defaultValue)
	if err != nil {
		return fmt.Errorf("failed to write prompt for '%s': %w", name, err)
	}

	userInput, err := s.waitForUserInput()
	if err != nil {
		return fmt.Errorf("failed to read user input for '%s': %w", name, err)
	}

	if userInput == "" {
		if required {
			_ = s.RenderError(fmt.Sprintf("This input '%s' is required. Please provide a value.\n", name))
			return s.renderInput(name, input)
		}

		input.Value = input.Default
	} else {
		input.Value = userInput
	}

	return nil
}

func (s *StdUI) waitForUserInput() (string, error) {
	s.stdin.Scan()

	// add new line after input
	_, err := s.stdout.WriteString("\n")
	if err != nil {
		return "", err
	}

	return s.stdin.Text(), s.stdin.Err()
}

// NewStdUI returns a ready-to-use StdUI backed by the provided file descriptors.
func NewStdUI(stdin *os.File, stdout *os.File, stderr *os.File) UI {
	return &StdUI{
		stdin:  bufio.NewScanner(stdin),
		stdout: stdout,
		stderr: stderr,
	}
}
