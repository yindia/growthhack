package cmd

import (
	"github.com/spf13/cobra"

	"growthhack/pkg/github"
)

func newGitHubCmd() *cobra.Command {
	var repoIn string
	var outPath string
	var workers int

	cmd := &cobra.Command{
		Use:   "github",
		Short: "Generate a CSV of GitHub users for growth hacking",
		RunE: func(cmd *cobra.Command, args []string) error {
			return github.Run(cmd.Context(), repoIn, outPath, workers)
		},
	}

	cmd.Flags().StringVar(&repoIn, "repo", "", "GitHub repo: owner/repo or https://github.com/owner/repo")
	cmd.Flags().StringVar(&outPath, "out", "users.csv", "Output CSV path")
	cmd.Flags().IntVar(&workers, "workers", 8, "Number of concurrent user detail fetch workers")
	_ = cmd.MarkFlagRequired("repo")

	return cmd
}
