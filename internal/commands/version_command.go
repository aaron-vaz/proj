package commands

import (
	"flag"
	"fmt"

	"github.com/aaron-vaz/skelly/internal/view"
)

// Version can be injected at build time using -ldflags "-X github.com/aaron-vaz/skelly/internal/cli.version=x.y.z"
var version = "dev"

// VersionCommand outputs the current build version of skelly.
type VersionCommand struct {
	ui view.UI
}

// Name returns the CLI identifier for this command.
func (c *VersionCommand) Name() string {
	return "version"
}

func (c *VersionCommand) Description() string {
	return "Show skelly version"
}

func (c *VersionCommand) Init(flags *flag.FlagSet) {}

// Run displays the injected skelly application version string to the user UI.
func (c *VersionCommand) Run(args []string) error {
	return c.ui.RenderInfo(fmt.Sprintf("skelly version %s", version))
}

// NewVersionCommand initializes and returns the version logger instance.
func NewVersionCommand(ui view.UI) Command[*flag.FlagSet] {
	return &VersionCommand{ui: ui}
}
