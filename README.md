# Growthhack

Growthhack is a small, focused CLI for collecting growth-hacking CSVs from GitHub and Reddit.
It helps you discover relevant users and conversations quickly, while keeping data collection reproducible.

## Features

- GitHub: export contributor, issue author, PR author, stargazer, and fork owner emails (public only)
- Reddit: search across topics with relevance scoring and write CSVs for outreach/analysis
- Hacker News: search stories via the Algolia API and export CSVs for outreach/analysis
- Fast, deterministic CSV output with clear column headers
- Simple CLI built with Cobra

## Install

```bash
# Run directly
go run . --help
```

> Requires Go 1.24+.

### Homebrew

```bash
brew install yindia/homebrew-yindia/growthhack
```

## Usage

### GitHub

```bash
# Export users from a repo
GITHUB_TOKEN=your_token \
  go run . github \
  --topic owner/repo \
  --topic another/repo \
  --out users.csv \
  --workers 8
```

Notes:
- `GITHUB_TOKEN` is optional but recommended to avoid rate limits.
- Only users with a public email are written to the CSV.
- When multiple repos are provided, the output filename is suffixed with the repo name.

### Reddit

```bash
# Search Reddit and write a CSV of relevant posts
go run . reddit \
  --topic "github webhook integration" \
  --topic "gitlab webhook receiver" \
  --subreddit webdev \
  --out reddit_posts.csv \
  --limit 200 \
  --minutes 1 \
  --sleep-ms 850 \
  --notify \
  --interval 30
```

Notes:
- `--notify` is off by default; pass `--notify` to keep polling and print new matches as they arrive.
- `--interval` controls the polling frequency (in seconds) for notify mode.

### Hacker News

```bash
# Search Hacker News and write a CSV of relevant stories
go run . hackernews \
  --topic "webhook integration" \
  --topic "github app webhook" \
  --out hackernews_posts.csv \
  --limit 200 \
  --minutes 1 \
  --sleep-ms 200 \
  --notify \
  --interval 30
```

Notes:
- `--notify` is off by default; pass `--notify` to keep polling and print new matches as they arrive.
- `--interval` controls the polling frequency (in seconds) for notify mode.

## CSV Outputs

### GitHub CSV

Columns:
- `username`
- `name`
- `email`
- `sources` (pipe-separated: contributor|issue_author|pr_author|stargazer|fork_owner)

### Reddit CSV

Columns:
- `relevance`
- `created_utc`
- `date_utc`
- `subreddit`
- `title`
- `author`
- `score`
- `num_comments`
- `providers`
- `matched_query`
- `sort`
- `permalink`
- `url`
- `selftext_excerpt`

### Hacker News CSV

Columns:
- `created_at`
- `created_at_i`
- `title`
- `author`
- `points`
- `num_comments`
- `url`
- `hn_url`
- `story_text_excerpt`
- `object_id`
- `query`

## Project Structure

```
cmd/                 CLI entrypoint and command wiring
pkg/github/           GitHub data collection logic
pkg/hackernews/       Hacker News data collection logic
pkg/reddit/           Reddit data collection logic
```

## Development

```bash
go test ./...
```

## Contributing

1. Fork the repo
2. Create a feature branch
3. Add tests for new behavior
4. Open a pull request

## License

MIT
