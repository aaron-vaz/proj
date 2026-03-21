package commands

import (
	"flag"
	"testing"
)

type mockCommand struct {
	name        string
	description string
	initFunc    func(flags *flag.FlagSet)
	runFunc     func(args []string) error
}

func (m *mockCommand) Name() string        { return m.name }
func (m *mockCommand) Description() string { return m.description }
func (m *mockCommand) Init(flags *flag.FlagSet) {
	if m.initFunc != nil {
		m.initFunc(flags)
	}
}
func (m *mockCommand) Run(args []string) error {
	if m.runFunc != nil {
		return m.runFunc(args)
	}
	return nil
}

func TestHelpCommand_Run(t *testing.T) {
	mockCmds := map[string]Command[*flag.FlagSet]{
		"dummy": &mockCommand{
			name:        "dummy",
			description: "A dummy command",
			initFunc: func(flags *flag.FlagSet) {
				flags.String("foo", "", "dummy flag")
			},
		},
	}

	cmd := NewHelpCommand(mockCmds)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no args shows help",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "known command shows help",
			args:    []string{"dummy"},
			wantErr: false,
		},
		{
			name:    "unknown command shows error",
			args:    []string{"unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			err := cmd.Run(tt.args)

			// Then
			if tt.wantErr {
				if err == nil {
					t.Errorf("Run() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}
