package hackernews

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type searchResponse struct {
	Hits    []hit `json:"hits"`
	Page    int   `json:"page"`
	NbPages int   `json:"nbPages"`
}

type hit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
	CreatedAt   string `json:"created_at"`
	CreatedAtI  int64  `json:"created_at_i"`
	StoryText   string `json:"story_text"`
}

type client struct {
	baseURL string
	http    *http.Client
	ua      string
}

func newClient() *client {
	return &client{
		baseURL: "https://hn.algolia.com/api/v1",
		http:    &http.Client{Timeout: 20 * time.Second},
		ua:      "growthhacks-hackernews/1.0",
	}
}

func (c *client) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
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

func buildSearchURL(query string, page int) string {
	v := url.Values{}
	v.Set("query", query)
	v.Set("tags", "story")
	v.Set("hitsPerPage", "100")
	v.Set("page", strconv.Itoa(page))
	return "/search?" + v.Encode()
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

func hnURL(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return "https://news.ycombinator.com/item?id=" + id
}

func Run(ctx context.Context, topic, outPath string, limit, days, sleepMs int) error {
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
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

	results := make([]hit, 0, limit)
	page := 0
	reqCount := 0

	for len(results) < limit {
		endpoint := buildSearchURL(topic, page)
		var resp searchResponse
		if err := client.getJSON(ctx, endpoint, &resp); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: search failed (page %d): %v\n", page, err)
			break
		}

		reqCount++
		if sleepMs > 0 {
			time.Sleep(time.Duration(sleepMs) * time.Millisecond)
		}

		if len(resp.Hits) == 0 {
			break
		}

		for _, h := range resp.Hits {
			created := time.Unix(h.CreatedAtI, 0)
			if created.Before(cutoff) {
				continue
			}
			results = append(results, h)
			if len(results) >= limit {
				break
			}
		}

		if resp.Page >= resp.NbPages-1 {
			break
		}
		page++
	}

	if len(results) == 0 {
		return fmt.Errorf("no results found")
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{
		"created_at",
		"created_at_i",
		"title",
		"author",
		"points",
		"num_comments",
		"url",
		"hn_url",
		"story_text_excerpt",
		"object_id",
		"query",
	}); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}

	for _, h := range results {
		createdAt := time.Unix(h.CreatedAtI, 0).UTC().Format(time.RFC3339)
		err := w.Write([]string{
			createdAt,
			strconv.FormatInt(h.CreatedAtI, 10),
			normalizeSpaces(h.Title),
			h.Author,
			strconv.Itoa(h.Points),
			strconv.Itoa(h.NumComments),
			h.URL,
			hnURL(h.ObjectID),
			truncate(h.StoryText, 240),
			h.ObjectID,
			topic,
		})
		if err != nil {
			return fmt.Errorf("writing CSV row: %w", err)
		}
	}

	if err := w.Error(); err != nil {
		return fmt.Errorf("writing CSV: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Done. Requests: %d | Wrote: %d -> %s\n",
		reqCount, len(results), outPath)
	return nil
}
