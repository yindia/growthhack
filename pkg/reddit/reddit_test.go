package reddit

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeSpaces(t *testing.T) {
	got := normalizeSpaces("hello\n  world\tfrom   reddit")
	if got != "hello world from reddit" {
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

func TestBuildOptimizedQueries(t *testing.T) {
	queries := buildOptimizedQueries("custom topic")

	expected := []string{"custom topic", "custom topic webhook", "custom topic github gitlab bitbucket"}
	for _, want := range expected {
		found := false
		for _, q := range queries {
			if q == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected query %q to be present", want)
		}
	}

	seen := map[string]bool{}
	for _, q := range queries {
		lower := strings.ToLower(q)
		if seen[lower] {
			t.Fatalf("duplicate query found: %q", q)
		}
		seen[lower] = true
	}

	dupeQueries := buildOptimizedQueries("github webhook integration")
	count := 0
	for _, q := range dupeQueries {
		if strings.EqualFold(q, "github webhook integration") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected de-duped query, got %d entries", count)
	}
}

func TestProviderFlags(t *testing.T) {
	got := providerFlags("GitHub and GitLab plus Bitbucket")
	if got != "github|gitlab|bitbucket" {
		t.Fatalf("unexpected provider flags: %q", got)
	}
}

func TestIsOnTopic(t *testing.T) {
	if !isOnTopic("GitHub webhook integration help", "building a webhook handler") {
		t.Fatalf("expected topic match")
	}

	if isOnTopic("Webhook integration", "no provider mentioned") {
		t.Fatalf("expected provider requirement to fail")
	}

	if isOnTopic("GitHub integration", "no hook mentioned") {
		t.Fatalf("expected webhook requirement to fail")
	}
}

func TestTokens(t *testing.T) {
	got := tokens("Hello, world!")
	want := []string{"hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected tokens: %#v", got)
	}
}
