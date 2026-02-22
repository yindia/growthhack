package cmd

import (
	"github.com/spf13/cobra"

	"growthhack/pkg/hackernews"
)

func newHackerNewsCmd() *cobra.Command {
	var topics []string
	var outPath string
	var limit int
	var minutes int
	var sleepMs int
	var intervalSeconds int
	var notify bool

	cmd := &cobra.Command{
		Use:   "hackernews",
		Short: "Search Hacker News and write a growth hacking CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			return hackernews.Run(cmd.Context(), topics, outPath, limit, minutes, sleepMs, intervalSeconds, notify)
		},
	}

	cmd.Flags().StringArrayVar(&topics, "topic", nil, "Search topic (repeatable)")
	cmd.Flags().StringVar(&outPath, "out", "hackernews_posts.csv", "Output CSV file path")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max number of results to write")
	cmd.Flags().IntVar(&minutes, "minutes", 1, "Only keep stories created within the last N minutes")
	cmd.Flags().IntVar(&sleepMs, "sleep-ms", 200, "Sleep between requests (ms) to be gentle with Algolia")
	cmd.Flags().IntVar(&intervalSeconds, "interval", 30, "Polling interval in seconds when --notify is enabled")
	cmd.Flags().BoolVar(&notify, "notify", false, "Enable notifications and continuous polling")
	_ = cmd.MarkFlagRequired("topic")

	return cmd
}
