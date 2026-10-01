package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCmd() *cobra.Command {
	return &cobra.Command{
		Use: "hello",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "hello")
		},
	}
}

func main() {
	if err := newCmd().Execute(); err != nil {
		panic(err)
	}
}
