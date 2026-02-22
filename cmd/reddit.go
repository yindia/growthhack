package cmd

import (
	"github.com/spf13/cobra"

	"growthhack/pkg/reddit"
)

func newRedditCmd() *cobra.Command {
	var topics []string
	var subreddit string
	var outPath string
	var limit int
	var minutes int
	var sleepMs int
	var intervalSeconds int
	var notify bool

	cmd := &cobra.Command{
		Use:   "reddit",
		Short: "Search Reddit and write a growth hacking CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			return reddit.Run(cmd.Context(), topics, subreddit, outPath, limit, minutes, sleepMs, intervalSeconds, notify)
		},
	}

	cmd.Flags().StringArrayVar(&topics, "topic", nil, "Topic to search for (repeatable)")
	cmd.Flags().StringVar(&subreddit, "subreddit", "", "Optional subreddit to restrict search (e.g. devops, webdev, golang)")
	cmd.Flags().StringVar(&outPath, "out", "reddit_posts.csv", "Output CSV file path")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max number of results to write (after filtering and dedupe)")
	cmd.Flags().IntVar(&minutes, "minutes", 1, "Only keep posts created within the last N minutes")
	cmd.Flags().IntVar(&sleepMs, "sleep-ms", 850, "Sleep between requests (ms) to be gentle with Reddit")
	cmd.Flags().IntVar(&intervalSeconds, "interval", 30, "Polling interval in seconds when --notify is enabled")
	cmd.Flags().BoolVar(&notify, "notify", false, "Enable notifications and continuous polling")
	_ = cmd.MarkFlagRequired("topic")

	return cmd
}
