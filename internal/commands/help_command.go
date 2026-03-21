package commands

import (
	"flag"
	"fmt"
)

// HelpCommand handles the display of usage instructions and command descriptions.
type HelpCommand struct {
	commands map[string]Command[*flag.FlagSet]
}

// Name returns the CLI identifier for this command.
func (c *HelpCommand) Name() string {
	return "help"
}

func (c *HelpCommand) Description() string {
	return "Show command help"
}

func (c *HelpCommand) Init(flags *flag.FlagSet) {}

// Run prints the application help structure or the specific help documentation
// for a requested sub-command.
func (c *HelpCommand) Run(args []string) error {
	if len(args) == 0 {
		c.showHelp()
		return nil
	}

	cmdName := args[0]
	cmd, ok := c.commands[cmdName]
	if !ok {
		c.showHelp()
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	flags := flag.NewFlagSet(cmd.Name(), flag.ContinueOnError)
	cmd.Init(flags)

	fmt.Printf("Usage: skelly %s [options]\n\n", cmd.Name())
	fmt.Println(cmd.Description())
	fmt.Println("\nOPTIONS:")
	flags.PrintDefaults()

	return nil
}

func (c *HelpCommand) showHelp() {
	fmt.Println("USAGE:")
	fmt.Println("    skelly COMMAND [OPTIONS]")
	fmt.Println("")
	fmt.Println("COMMANDS:")
	for _, cmd := range c.commands {
		fmt.Printf("    %-11s %s\n", cmd.Name(), cmd.Description())
	}
	fmt.Println("")
	fmt.Println("Run 'skelly help COMMAND' for command-specific help")
}

// NewHelpCommand returns a configured HelpCommand that understands the
// full map of available commands in order to print their documentation.
func NewHelpCommand(commands map[string]Command[*flag.FlagSet]) Command[*flag.FlagSet] {
	return &HelpCommand{
		commands: commands,
	}
}
