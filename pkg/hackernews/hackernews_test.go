package hackernews

import (
	"strings"
	"testing"
)

func TestBuildSearchURL(t *testing.T) {
	endpoint := buildSearchURL("hello world", 2)
	if !strings.HasPrefix(endpoint, "/search?") {
		t.Fatalf("unexpected endpoint prefix: %q", endpoint)
	}
	if !strings.Contains(endpoint, "query=hello+world") {
		t.Fatalf("missing query param: %q", endpoint)
	}
	if !strings.Contains(endpoint, "page=2") {
		t.Fatalf("missing page param: %q", endpoint)
	}
	if !strings.Contains(endpoint, "tags=story") {
		t.Fatalf("missing tags param: %q", endpoint)
	}
}

func TestNormalizeSpaces(t *testing.T) {
	got := normalizeSpaces("hello\n  world\tfrom   hn")
	if got != "hello world from hn" {
		t.Fatalf("unexpected normalized value: %q", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Fatalf("unexpected truncate result: %q", got)
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Fatalf("unexpected truncate result: %q", got)
	}
}

func TestHNURL(t *testing.T) {
	if got := hnURL(""); got != "" {
		t.Fatalf("expected empty hn url, got %q", got)
	}
	if got := hnURL("123"); got != "https://news.ycombinator.com/item?id=123" {
		t.Fatalf("unexpected hn url: %q", got)
	}
}
