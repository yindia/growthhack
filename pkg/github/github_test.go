package github

import "testing"

func TestParseRepoOwnerRepo(t *testing.T) {
	owner, repo, err := parseRepo("foo/bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "foo" || repo != "bar" {
		t.Fatalf("unexpected owner/repo: %s/%s", owner, repo)
	}
}

func TestParseRepoURL(t *testing.T) {
	owner, repo, err := parseRepo("https://github.com/foo/bar/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "foo" || repo != "bar" {
		t.Fatalf("unexpected owner/repo: %s/%s", owner, repo)
	}
}

func TestParseRepoInvalid(t *testing.T) {
	cases := []string{"", "foo", "foo/bar/baz", "https://github.com/foo"}
	for _, input := range cases {
		if _, _, err := parseRepo(input); err == nil {
			t.Fatalf("expected error for %q", input)
		}
	}
}

func TestNextLink(t *testing.T) {
	link := `<https://api.github.com/resource?page=2>; rel="next", <https://api.github.com/resource?page=5>; rel="last"`
	next := nextLink(link)
	if next != "https://api.github.com/resource?page=2" {
		t.Fatalf("unexpected next link: %q", next)
	}

	if nextLink("") != "" {
		t.Fatalf("expected empty next link")
	}
}
