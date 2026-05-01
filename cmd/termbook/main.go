package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:           "termbook",
		Short:         "Beautiful, browsable reference pages for your CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.AddCommand(newRecordCmd(), newBuildCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "termbook:", err)
		os.Exit(1)
	}
}
