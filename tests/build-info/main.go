package main

import (
	"example.com/lib"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:     lib.Name,
		Aliases: lo.Uniq([]string{lib.Name, lib.Name}),
	}
	cobra.CheckErr(cmd.Execute())
}
