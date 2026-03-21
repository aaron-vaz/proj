package cli

import (
	"errors"
	"flag"
	"testing"
	"strings"

	"github.com/aaron-vaz/skelly/internal/commands"
)

type mockCommand struct {
	name        string
	description string
	initFunc    func(flags *flag.FlagSet)
	runFunc     func(args []string) error
}

func (m *mockCommand) Name() string {
	return m.name
}

func (m *mockCommand) Description() string {
	return m.description
}

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

func TestFlagCommandInvoker_Execute(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		setupCmds func() map[string]commands.Command[*flag.FlagSet]
		wantErr   bool
		errMsg    string
	}{
		{
			name: "execute help when no args provided",
			args: []string{},
			setupCmds: func() map[string]commands.Command[*flag.FlagSet] {
				return map[string]commands.Command[*flag.FlagSet]{
					"help": &mockCommand{
						name: "help",
						runFunc: func(args []string) error {
							return nil
						},
					},
				}
			},
			wantErr: false,
		},
		{
			name: "execute known command",
			args: []string{"testcmd", "--foo", "bar"},
			setupCmds: func() map[string]commands.Command[*flag.FlagSet] {
				return map[string]commands.Command[*flag.FlagSet]{
					"testcmd": &mockCommand{
						name: "testcmd",
						initFunc: func(flags *flag.FlagSet) {
							flags.String("foo", "", "foo flag")
						},
						runFunc: func(args []string) error {
							if len(args) != 0 {
								return errors.New("expected no extra args after parsing foo")
							}
							return nil
						},
					},
				}
			},
			wantErr: false,
		},
		{
			name: "unknown command",
			args: []string{"unknowncmd"},
			setupCmds: func() map[string]commands.Command[*flag.FlagSet] {
				return map[string]commands.Command[*flag.FlagSet]{}
			},
			wantErr: true,
			errMsg:  "unknown command: unknowncmd",
		},
		{
			name: "flag parsing error",
			args: []string{"testcmd", "--unknownFlag"},
			setupCmds: func() map[string]commands.Command[*flag.FlagSet] {
				return map[string]commands.Command[*flag.FlagSet]{
					"testcmd": &mockCommand{
						name: "testcmd",
						initFunc: func(flags *flag.FlagSet) {
							// Disable flag output during tests
							flags.SetOutput(new(strings.Builder))
						},
					},
				}
			},
			wantErr: true,
			errMsg:  "failed to parse flags for command 'testcmd': flag provided but not defined: -unknownFlag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			cmds := tt.setupCmds()
			invoker := &FlagCommandInvoker{
				commands: cmds,
			}

			// When
			err := invoker.Execute(tt.args)
			
			// Then
			if tt.wantErr {
				if err == nil {
					t.Errorf("Execute() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Execute() error = %v, wantErrMsg %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}
