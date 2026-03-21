package main

import (
	"fmt"
	"os"

	"github.com/aaron-vaz/skelly/internal/cli"
	"github.com/aaron-vaz/skelly/internal/download"
	"github.com/aaron-vaz/skelly/internal/templates"
	"github.com/aaron-vaz/skelly/internal/view"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	downloader := download.NewSmartDownloader(
		download.NewLocalDownloader(),
		download.NewGoGetterDownloader(),
	)
	renderer := templates.NewRendererService()
	processor := templates.NewTemplateProcessor(renderer)
	ui := view.NewStdUI(os.Stdin, os.Stdout, os.Stderr)
	invoker := cli.NewFlagCommandInvoker(downloader, processor, ui)

	return invoker.Execute(os.Args[1:])
}
