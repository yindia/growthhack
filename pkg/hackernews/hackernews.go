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
	"sort"
	"strconv"
	"strings"
	"time"
)

var hnCSVHeader = []string{
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
}

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

type result struct {
	Hit   hit
	Query string
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

func fetchResults(ctx context.Context, topics []string, limit, minutes, sleepMs int) ([]result, int) {
	client := newClient()
	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)

	resultsByID := map[string]result{}
	reqCount := 0

	for _, topic := range topics {
		page := 0
		for len(resultsByID) < limit {
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
				if len(resultsByID) >= limit {
					break
				}
				created := time.Unix(h.CreatedAtI, 0)
				if created.Before(cutoff) {
					continue
				}
				if _, ok := resultsByID[h.ObjectID]; ok {
					continue
				}
				resultsByID[h.ObjectID] = result{Hit: h, Query: topic}
			}

			if resp.Page >= resp.NbPages-1 {
				break
			}
			page++
		}

		if len(resultsByID) >= limit {
			break
		}
	}

	results := make([]result, 0, len(resultsByID))
	for _, r := range resultsByID {
		results = append(results, r)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Hit.CreatedAtI > results[j].Hit.CreatedAtI
	})

	return results, reqCount
}

func writeCSV(outPath string, results []result) error {
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write(hnCSVHeader); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}

	if err := writeRows(w, results); err != nil {
		return err
	}

	if err := w.Error(); err != nil {
		return fmt.Errorf("writing CSV: %w", err)
	}

	return nil
}

func writeRows(w *csv.Writer, results []result) error {
	for _, r := range results {
		h := r.Hit
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
			r.Query,
		})
		if err != nil {
			return fmt.Errorf("writing CSV row: %w", err)
		}
	}
	return nil
}

func openAppendWriter(outPath string) (*os.File, *csv.Writer, error) {
	var size int64
	if info, err := os.Stat(outPath); err == nil {
		size = info.Size()
	} else if !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("stat output file: %w", err)
	}

	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("opening output file: %w", err)
	}

	w := csv.NewWriter(f)
	if size == 0 {
		if err := w.Write(hnCSVHeader); err != nil {
			f.Close()
			return nil, nil, fmt.Errorf("writing CSV header: %w", err)
		}
		w.Flush()
		if err := w.Error(); err != nil {
			f.Close()
			return nil, nil, fmt.Errorf("writing CSV header: %w", err)
		}
	}

	return f, w, nil
}

func Run(ctx context.Context, topics []string, outPath string, limit, minutes, sleepMs, intervalSeconds int, notify bool) error {
	cleanTopics := make([]string, 0, len(topics))
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic != "" {
			cleanTopics = append(cleanTopics, topic)
		}
	}
	if len(cleanTopics) == 0 {
		return fmt.Errorf("topic is required")
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 2000 {
		limit = 2000
	}
	if minutes < 1 {
		minutes = 1
	}
	if sleepMs < 0 {
		sleepMs = 0
	}
	if intervalSeconds < 1 {
		intervalSeconds = 30
	}

	if !notify {
		results, reqCount := fetchResults(ctx, cleanTopics, limit, minutes, sleepMs)
		if len(results) == 0 {
			return fmt.Errorf("no results found")
		}
		if err := writeCSV(outPath, results); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Done. Requests: %d | Wrote: %d -> %s\n",
			reqCount, len(results), outPath)
		return nil
	}

	f, w, err := openAppendWriter(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	defer w.Flush()

	seen := map[string]bool{}
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		results, _ := fetchResults(ctx, cleanTopics, limit, minutes, sleepMs)
		newOnes := make([]result, 0, len(results))
		for _, r := range results {
			if seen[r.Hit.ObjectID] {
				continue
			}
			seen[r.Hit.ObjectID] = true
			newOnes = append(newOnes, r)
		}

		sort.Slice(newOnes, func(i, j int) bool {
			return newOnes[i].Hit.CreatedAtI < newOnes[j].Hit.CreatedAtI
		})

		for _, r := range newOnes {
			fmt.Fprintf(os.Stderr, "Match: %s | %s\n", r.Hit.Title, hnURL(r.Hit.ObjectID))
		}

		if len(newOnes) > 0 {
			if err := writeRows(w, newOnes); err != nil {
				return err
			}
			w.Flush()
			if err := w.Error(); err != nil {
				return fmt.Errorf("writing CSV: %w", err)
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
