package cli

import (
	"flag"
	"fmt"

	"github.com/aaron-vaz/skelly/internal/commands"
	"github.com/aaron-vaz/skelly/internal/download"
	"github.com/aaron-vaz/skelly/internal/templates"
	"github.com/aaron-vaz/skelly/internal/view"
)

// FlagCommandInvoker is an implementation of commands.Invoker that maps
// CLI commands to their respective handlers and parses flag inputs.
type FlagCommandInvoker struct {
	commands map[string]commands.Command[*flag.FlagSet]
}

// Execute parses the provided arguments, determines the target command,
// parses associated flags, and executes the command's Run logic.
func (i *FlagCommandInvoker) Execute(args []string) error {
	if len(args) < 1 {
		return i.commands["help"].Run(nil)
	}

	cmdName := args[0]
	cmd, ok := i.commands[cmdName]
	if !ok {
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	flags := flag.NewFlagSet(cmd.Name(), flag.ContinueOnError)
	cmd.Init(flags)

	if err := flags.Parse(args[1:]); err != nil {
		return fmt.Errorf("failed to parse flags for command '%s': %w", cmd.Name(), err)
	}

	return cmd.Run(flags.Args())
}

// NewFlagCommandInvoker creates and configures a new FlagCommandInvoker with
// all supported CLI commands registered automatically.
func NewFlagCommandInvoker(
	downloader download.Downloader,
	processor *templates.Processor,
	ui view.UI,
) commands.Invoker {
	invoker := &FlagCommandInvoker{
		commands: make(map[string]commands.Command[*flag.FlagSet]),
	}

	cmds := []commands.Command[*flag.FlagSet]{
		commands.NewInitCommand(processor, downloader, ui),
		commands.NewHelpCommand(invoker.commands),
		commands.NewVersionCommand(ui),
	}

	for _, cmd := range cmds {
		invoker.commands[cmd.Name()] = cmd
	}

	return invoker
}
