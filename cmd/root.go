package cmd

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "growthhacks",
		Short:        "Growth hacking data collection CLI",
		SilenceUsage: true,
	}

	cmd.AddCommand(newGitHubCmd(), newRedditCmd(), newHackerNewsCmd())

	return cmd
}
