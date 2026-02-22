package cmd

import (
	"github.com/spf13/cobra"

	"growthhack/pkg/reddit"
)

func newRedditCmd() *cobra.Command {
	var topic string
	var subreddit string
	var outPath string
	var limit int
	var days int
	var sleepMs int

	cmd := &cobra.Command{
		Use:   "reddit",
		Short: "Search Reddit and write a growth hacking CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			return reddit.Run(cmd.Context(), topic, subreddit, outPath, limit, days, sleepMs)
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Topic to search for (e.g. 'github gitlab bitbucket webhook integration')")
	cmd.Flags().StringVar(&subreddit, "subreddit", "", "Optional subreddit to restrict search (e.g. devops, webdev, golang)")
	cmd.Flags().StringVar(&outPath, "out", "reddit_posts.csv", "Output CSV file path")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max number of results to write (after filtering and dedupe)")
	cmd.Flags().IntVar(&days, "days", 365, "Only keep posts created within the last N days")
	cmd.Flags().IntVar(&sleepMs, "sleep-ms", 850, "Sleep between requests (ms) to be gentle with Reddit")
	_ = cmd.MarkFlagRequired("topic")

	return cmd
}
