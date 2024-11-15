package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "server",
	}
	cmd.AddCommand(runCmd())
	cmd.SilenceUsage = true

	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
