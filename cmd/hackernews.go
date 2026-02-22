package cmd

import (
	"github.com/spf13/cobra"

	"growthhack/pkg/hackernews"
)

func newHackerNewsCmd() *cobra.Command {
	var topic string
	var outPath string
	var limit int
	var days int
	var sleepMs int

	cmd := &cobra.Command{
		Use:   "hackernews",
		Short: "Search Hacker News and write a growth hacking CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			return hackernews.Run(cmd.Context(), topic, outPath, limit, days, sleepMs)
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Search topic (e.g. 'webhook integration')")
	cmd.Flags().StringVar(&outPath, "out", "hackernews_posts.csv", "Output CSV file path")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max number of results to write")
	cmd.Flags().IntVar(&days, "days", 365, "Only keep stories created within the last N days")
	cmd.Flags().IntVar(&sleepMs, "sleep-ms", 200, "Sleep between requests (ms) to be gentle with Algolia")
	_ = cmd.MarkFlagRequired("topic")

	return cmd
}
