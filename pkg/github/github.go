package github

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type userRow struct {
	Login   string
	Name    string
	Email   string
	Sources map[string]bool
}

type client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func newClient(token string) *client {
	return &client{
		baseURL: "https://api.github.com",
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *client) newRequest(ctx context.Context, method, endpoint string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "gh-repo-users-csv-cli")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

func (c *client) doJSON(ctx context.Context, method, endpoint string, out any) (http.Header, error) {
	for {
		req, err := c.newRequest(ctx, method, endpoint)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return resp.Header, err
		}

		if resp.StatusCode == 403 {
			reset := resp.Header.Get("X-RateLimit-Reset")
			remaining := resp.Header.Get("X-RateLimit-Remaining")
			if remaining == "0" && reset != "" {
				if unix, err := strconv.ParseInt(reset, 10, 64); err == nil {
					sleep := time.Until(time.Unix(unix, 0)) + 2*time.Second
					if sleep > 0 && sleep < 10*time.Minute {
						fmt.Fprintf(os.Stderr, "Rate limited. Sleeping %s...\n", sleep.Round(time.Second))
						time.Sleep(sleep)
						continue
					}
				}
			}
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return resp.Header, fmt.Errorf("GitHub API error %d for %s: %s",
				resp.StatusCode, endpoint, strings.TrimSpace(string(bodyBytes)))
		}

		dec := json.NewDecoder(strings.NewReader(string(bodyBytes)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(out); err != nil {
			if err2 := json.Unmarshal(bodyBytes, out); err2 != nil {
				return resp.Header, err2
			}
		}

		return resp.Header, nil
	}
}

var linkRe = regexp.MustCompile(`<([^>]+)>;\s*rel="([^"]+)"`)

func nextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}
	parts := strings.Split(linkHeader, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		m := linkRe.FindStringSubmatch(p)
		if len(m) == 3 && m[2] == "next" {
			return m[1]
		}
	}
	return ""
}

func (c *client) paginateGET(ctx context.Context, endpoint string, handlePage func(raw json.RawMessage) error) error {
	for endpoint != "" {
		var raw json.RawMessage
		hdr, err := c.doJSON(ctx, http.MethodGet, endpoint, &raw)
		if err != nil {
			return err
		}
		if err := handlePage(raw); err != nil {
			return err
		}

		n := nextLink(hdr.Get("Link"))
		if n == "" {
			break
		}
		u, err := url.Parse(n)
		if err != nil {
			break
		}
		endpoint = u.Path
		if u.RawQuery != "" {
			endpoint += "?" + u.RawQuery
		}
	}
	return nil
}

func parseRepo(input string) (owner, repo string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", fmt.Errorf("repo input is empty")
	}

	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		u, e := url.Parse(input)
		if e != nil {
			return "", "", fmt.Errorf("invalid repo url: %w", e)
		}
		path := strings.Trim(u.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			return "", "", fmt.Errorf("repo url must look like https://github.com/owner/repo")
		}
		return parts[0], parts[1], nil
	}

	parts := strings.Split(strings.Trim(input, "/"), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repo must be in form owner/repo or https://github.com/owner/repo")
	}
	return parts[0], parts[1], nil
}

func outputPathForRepo(basePath, owner, repo string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return ""
	}

	if info, err := os.Stat(basePath); err == nil && info.IsDir() {
		return filepath.Join(basePath, fmt.Sprintf("%s-%s.csv", owner, repo))
	}

	ext := filepath.Ext(basePath)
	if ext == "" {
		return fmt.Sprintf("%s-%s-%s.csv", basePath, owner, repo)
	}

	dir := filepath.Dir(basePath)
	base := strings.TrimSuffix(filepath.Base(basePath), ext)
	return filepath.Join(dir, fmt.Sprintf("%s-%s-%s%s", base, owner, repo, ext))
}

func Run(ctx context.Context, repos []string, outPath string, workers int) error {
	if len(repos) == 0 {
		return fmt.Errorf("repo is required")
	}

	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	client := newClient(token)
	multi := len(repos) > 1

	for _, repoIn := range repos {
		repoIn = strings.TrimSpace(repoIn)
		if repoIn == "" {
			return fmt.Errorf("repo is required")
		}

		owner, repo, err := parseRepo(repoIn)
		if err != nil {
			return err
		}

		targetOut := outPath
		if multi {
			targetOut = outputPathForRepo(outPath, owner, repo)
		}
		if targetOut == "" {
			return fmt.Errorf("out path is required")
		}

		if err := runRepo(ctx, client, owner, repo, targetOut, workers); err != nil {
			return err
		}
	}

	return nil
}

func runRepo(ctx context.Context, client *client, owner, repo, outPath string, workers int) error {
	repoLabel := fmt.Sprintf("%s/%s", owner, repo)

	users := map[string]*userRow{}
	addUser := func(login, source string) {
		login = strings.TrimSpace(login)
		if login == "" {
			return
		}
		u, ok := users[login]
		if !ok {
			u = &userRow{
				Login:   login,
				Sources: map[string]bool{},
			}
			users[login] = u
		}
		u.Sources[source] = true
	}

	perPage := 100

	fmt.Fprintf(os.Stderr, "Fetching contributors for %s...\n", repoLabel)
	if err := client.paginateGET(ctx,
		fmt.Sprintf("/repos/%s/%s/contributors?per_page=%d&anon=false", owner, repo, perPage),
		func(raw json.RawMessage) error {
			var arr []struct {
				Login string `json:"login"`
			}
			if err := json.Unmarshal(raw, &arr); err != nil {
				return err
			}
			for _, it := range arr {
				addUser(it.Login, "contributor")
			}
			return nil
		},
	); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: contributors fetch failed: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "Fetching issues (authors) for %s...\n", repoLabel)
	if err := client.paginateGET(ctx,
		fmt.Sprintf("/repos/%s/%s/issues?state=all&per_page=%d", owner, repo, perPage),
		func(raw json.RawMessage) error {
			var arr []struct {
				User struct {
					Login string `json:"login"`
				} `json:"user"`
				PullRequest *json.RawMessage `json:"pull_request"`
			}
			if err := json.Unmarshal(raw, &arr); err != nil {
				return err
			}
			for _, it := range arr {
				if it.PullRequest != nil {
					continue
				}
				addUser(it.User.Login, "issue_author")
			}
			return nil
		},
	); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: issues fetch failed: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "Fetching pull requests (authors) for %s...\n", repoLabel)
	if err := client.paginateGET(ctx,
		fmt.Sprintf("/repos/%s/%s/pulls?state=all&per_page=%d", owner, repo, perPage),
		func(raw json.RawMessage) error {
			var arr []struct {
				User struct {
					Login string `json:"login"`
				} `json:"user"`
			}
			if err := json.Unmarshal(raw, &arr); err != nil {
				return err
			}
			for _, it := range arr {
				addUser(it.User.Login, "pr_author")
			}
			return nil
		},
	); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: pull requests fetch failed: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "Fetching stargazers for %s...\n", repoLabel)
	if err := client.paginateGET(ctx,
		fmt.Sprintf("/repos/%s/%s/stargazers?per_page=%d", owner, repo, perPage),
		func(raw json.RawMessage) error {
			var arr []struct {
				Login string `json:"login"`
			}
			if err := json.Unmarshal(raw, &arr); err != nil {
				return err
			}
			for _, it := range arr {
				addUser(it.Login, "stargazer")
			}
			return nil
		},
	); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: stargazers fetch failed: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "Fetching forks (owners) for %s...\n", repoLabel)
	if err := client.paginateGET(ctx,
		fmt.Sprintf("/repos/%s/%s/forks?per_page=%d", owner, repo, perPage),
		func(raw json.RawMessage) error {
			var arr []struct {
				Owner struct {
					Login string `json:"login"`
				} `json:"owner"`
			}
			if err := json.Unmarshal(raw, &arr); err != nil {
				return err
			}
			for _, it := range arr {
				addUser(it.Owner.Login, "fork_owner")
			}
			return nil
		},
	); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: forks fetch failed: %v\n", err)
	}

	if len(users) == 0 {
		return fmt.Errorf("no users found (repo may be empty, or API calls failed)")
	}

	fmt.Fprintf(os.Stderr, "Fetching user details for %d users (%s)...\n", len(users), repoLabel)

	type ghUser struct {
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	logins := make([]string, 0, len(users))
	for login := range users {
		logins = append(logins, login)
	}
	sort.Strings(logins)

	jobs := make(chan string)
	var wg sync.WaitGroup
	var mu sync.Mutex

	workerFn := func() {
		defer wg.Done()
		for login := range jobs {
			var u ghUser
			_, err := client.doJSON(ctx, http.MethodGet, "/users/"+url.PathEscape(login), &u)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to fetch /users/%s: %v\n", login, err)
				continue
			}

			if strings.TrimSpace(u.Email) == "" {
				mu.Lock()
				delete(users, login)
				mu.Unlock()
				continue
			}

			mu.Lock()
			row := users[login]
			if row != nil {
				row.Name = u.Name
				row.Email = u.Email
			}
			mu.Unlock()
		}
	}

	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go workerFn()
	}

	for _, login := range logins {
		jobs <- login
	}
	close(jobs)
	wg.Wait()

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"username", "name", "email", "sources"}); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}

	written := 0
	for _, login := range logins {
		row := users[login]
		if row == nil || strings.TrimSpace(row.Email) == "" {
			continue
		}
		srcs := make([]string, 0, len(row.Sources))
		for s := range row.Sources {
			srcs = append(srcs, s)
		}
		sort.Strings(srcs)

		if err := w.Write([]string{
			row.Login,
			row.Name,
			row.Email,
			strings.Join(srcs, "|"),
		}); err != nil {
			return fmt.Errorf("writing CSV row: %w", err)
		}
		written++
	}

	if err := w.Error(); err != nil {
		return fmt.Errorf("writing CSV: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Done. Wrote %d rows to %s\n", written, outPath)
	return nil
}
