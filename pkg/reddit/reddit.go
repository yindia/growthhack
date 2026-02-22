package reddit

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type listing struct {
	Data struct {
		After    string  `json:"after"`
		Children []child `json:"children"`
	} `json:"data"`
}

type child struct {
	Kind string `json:"kind"`
	Data post   `json:"data"`
}

type post struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Subreddit   string  `json:"subreddit"`
	Author      string  `json:"author"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	CreatedUTC  float64 `json:"created_utc"`
	Permalink   string  `json:"permalink"`
	URL         string  `json:"url"`
	SelfText    string  `json:"selftext"`
	IsSelf      bool    `json:"is_self"`
}

type result struct {
	Post          post
	MatchedQuery  string
	Sort          string
	Relevance     int
	ProviderFlags string
}

type client struct {
	http *http.Client
	ua   string
}

func newClient() *client {
	return &client{
		http: &http.Client{Timeout: 25 * time.Second},
		ua:   "reddit-webhook-integration-search/1.0 (contact: you@example.com)",
	}
}

func (c *client) getJSON(ctx context.Context, fullURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func normalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, max int) string {
	s = normalizeSpaces(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func buildOptimizedQueries(topic string) []string {
	topic = strings.TrimSpace(topic)

	baseSeeds := []string{
		"github webhook integration",
		"gitlab webhook integration",
		"bitbucket webhook integration",
		"build webhook receiver github",
		"webhook signature verification github",
		"gitlab webhook events integration",
		"bitbucket webhooks build service",
		"repository integration webhook system",
		"event subscription webhook handler",
		"webhook delivery retry queue",
		"webhook idempotency",
		"webhook security hmac signature",
		"github app webhook build",
		"gitlab system hooks build",
		"bitbucket app password webhook",
	}

	var userVariants []string
	if topic != "" {
		userVariants = append(userVariants, topic)

		if !strings.Contains(strings.ToLower(topic), "webhook") {
			userVariants = append(userVariants, topic+" webhook")
		}

		l := strings.ToLower(topic)
		if !strings.Contains(l, "github") && !strings.Contains(l, "gitlab") && !strings.Contains(l, "bitbucket") {
			userVariants = append(userVariants, topic+" github gitlab bitbucket")
		}
	}

	phraseVariants := []string{
		"\"webhook receiver\" github",
		"\"webhook handler\" github integration",
		"\"webhook signature\" github",
		"\"github app\" webhook",
		"\"gitlab system hooks\"",
		"\"bitbucket webhook\" build",
		"\"incoming webhook\" integration",
	}

	queries := append([]string{}, userVariants...)
	queries = append(queries, baseSeeds...)
	queries = append(queries, phraseVariants...)

	seen := map[string]bool{}
	out := make([]string, 0, len(queries))
	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		qLower := strings.ToLower(q)
		if seen[qLower] {
			continue
		}
		seen[qLower] = true
		out = append(out, q)
	}
	return out
}

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

func tokens(s string) []string {
	s = strings.ToLower(s)
	s = nonWord.ReplaceAllString(s, " ")
	parts := strings.Fields(s)
	return parts
}

func containsAny(text string, words []string) bool {
	tset := map[string]bool{}
	for _, t := range tokens(text) {
		tset[t] = true
	}
	for _, w := range words {
		if tset[strings.ToLower(w)] {
			return true
		}
	}
	return false
}

func providerFlags(text string) string {
	l := strings.ToLower(text)
	flags := []string{}
	if strings.Contains(l, "github") {
		flags = append(flags, "github")
	}
	if strings.Contains(l, "gitlab") {
		flags = append(flags, "gitlab")
	}
	if strings.Contains(l, "bitbucket") {
		flags = append(flags, "bitbucket")
	}
	return strings.Join(flags, "|")
}

func isOnTopic(title, body string) bool {
	text := title + " " + body

	hasWebhook := containsAny(text, []string{"webhook", "webhooks"})
	hasProvider := containsAny(text, []string{"github", "gitlab", "bitbucket"})
	hasBuildish := containsAny(text, []string{
		"integrate", "integration", "build", "implement", "setup", "set", "configure", "receiver", "handler",
		"endpoint", "api", "service", "pipeline", "automation", "events", "payload", "signature", "hmac",
	})

	return hasWebhook && hasProvider && hasBuildish
}

func relevanceScore(p post, matchedQuery string) (score int, flags string) {
	text := strings.ToLower(p.Title + " " + p.SelfText)
	flags = providerFlags(text)

	boost := func(term string, pts int) {
		if strings.Contains(text, term) {
			score += pts
		}
	}
	boost("webhook", 10)
	boost("integration", 8)
	boost("integrate", 6)
	boost("receiver", 6)
	boost("handler", 6)
	boost("endpoint", 4)
	boost("signature", 6)
	boost("hmac", 6)
	boost("retry", 4)
	boost("idempot", 4)
	boost("queue", 3)
	boost("events", 3)
	boost("payload", 3)

	boost("github", 6)
	boost("gitlab", 6)
	boost("bitbucket", 6)

	q := strings.ToLower(matchedQuery)
	for _, w := range []string{"webhook", "github", "gitlab", "bitbucket", "integration"} {
		if strings.Contains(q, w) && strings.Contains(text, w) {
			score += 2
		}
	}

	ageDays := int(time.Since(time.Unix(int64(p.CreatedUTC), 0)).Hours() / 24)
	switch {
	case ageDays <= 7:
		score += 8
	case ageDays <= 30:
		score += 5
	case ageDays <= 180:
		score += 2
	}

	if p.Score > 50 {
		score += 3
	}
	if p.NumComments > 20 {
		score += 3
	}

	return score, flags
}

func buildSearchURL(subreddit, query, sort, after string) string {
	base := "https://www.reddit.com/search.json"
	if subreddit != "" {
		base = fmt.Sprintf("https://www.reddit.com/r/%s/search.json", url.PathEscape(subreddit))
	}

	v := url.Values{}
	v.Set("q", query)
	v.Set("sort", sort)
	v.Set("t", "all")
	v.Set("limit", "100")
	v.Set("raw_json", "1")
	if subreddit != "" {
		v.Set("restrict_sr", "1")
	}
	if after != "" {
		v.Set("after", after)
	}
	return base + "?" + v.Encode()
}

func Run(ctx context.Context, topic, subreddit, outPath string, limit, days, sleepMs int) error {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return fmt.Errorf("topic is required")
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 2000 {
		limit = 2000
	}
	if days < 1 {
		days = 365
	}
	if sleepMs < 0 {
		sleepMs = 0
	}

	client := newClient()
	queries := buildOptimizedQueries(topic)
	sorts := []string{"relevance", "new"}

	resultsByID := map[string]result{}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

	reqCount := 0

	fmt.Fprintf(os.Stderr, "Running %d optimized queries across %d sorts...\n", len(queries), len(sorts))
	for _, q := range queries {
		for _, sortMode := range sorts {
			after := ""
			for {
				if len(resultsByID) >= limit*4 {
					break
				}

				searchURL := buildSearchURL(subreddit, q, sortMode, after)
				var listing listing
				if err := client.getJSON(ctx, searchURL, &listing); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: search failed (%s | %s): %v\n", sortMode, q, err)
					break
				}

				reqCount++
				if sleepMs > 0 {
					time.Sleep(time.Duration(sleepMs) * time.Millisecond)
				}

				if len(listing.Data.Children) == 0 {
					break
				}

				for _, ch := range listing.Data.Children {
					p := ch.Data
					created := time.Unix(int64(p.CreatedUTC), 0)
					if created.Before(cutoff) {
						continue
					}

					if !isOnTopic(p.Title, p.SelfText) {
						continue
					}

					score, flags := relevanceScore(p, q)

					existing, ok := resultsByID[p.ID]
					if !ok || score > existing.Relevance {
						resultsByID[p.ID] = result{
							Post:          p,
							MatchedQuery:  q,
							Sort:          sortMode,
							Relevance:     score,
							ProviderFlags: flags,
						}
					}
				}

				after = listing.Data.After
				if after == "" {
					break
				}

				if reqCount > 250 {
					break
				}
			}
		}
	}

	all := make([]result, 0, len(resultsByID))
	for _, r := range resultsByID {
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Relevance != all[j].Relevance {
			return all[i].Relevance > all[j].Relevance
		}
		return all[i].Post.CreatedUTC > all[j].Post.CreatedUTC
	})

	if len(all) > limit {
		all = all[:limit]
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{
		"relevance",
		"created_utc",
		"date_utc",
		"subreddit",
		"title",
		"author",
		"score",
		"num_comments",
		"providers",
		"matched_query",
		"sort",
		"permalink",
		"url",
		"selftext_excerpt",
	}); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}

	for _, r := range all {
		p := r.Post
		created := time.Unix(int64(p.CreatedUTC), 0).UTC()
		permalink := "https://www.reddit.com" + p.Permalink

		err := w.Write([]string{
			strconv.Itoa(r.Relevance),
			strconv.FormatInt(int64(p.CreatedUTC), 10),
			created.Format(time.RFC3339),
			p.Subreddit,
			normalizeSpaces(p.Title),
			p.Author,
			strconv.Itoa(p.Score),
			strconv.Itoa(p.NumComments),
			r.ProviderFlags,
			r.MatchedQuery,
			r.Sort,
			permalink,
			p.URL,
			truncate(p.SelfText, 240),
		})
		if err != nil {
			return fmt.Errorf("writing CSV row: %w", err)
		}
	}

	if err := w.Error(); err != nil {
		return fmt.Errorf("writing CSV: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Done. Requests: %d | Candidates: %d | Wrote: %d -> %s\n",
		reqCount, len(resultsByID), len(all), outPath)
	return nil
}
