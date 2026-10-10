package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestResolveFeed(t *testing.T) {
	feed := TaggedFeed("me/app", "desktop-v", "update-linux-amd64.json")
	if want := "https://github.com/me/app/releases/download/desktop-v{version}/update-linux-amd64.json"; feed != want {
		t.Fatalf("feed = %s, want %s", feed, want)
	}
	// The releases of the repository, newest first, a page for each list.
	var pages [][]string
	get := func(_ context.Context, u string) (io.ReadCloser, error) {
		for i, page := range pages {
			if u == fmt.Sprintf("https://api.github.com/repos/me/app/releases?per_page=100&page=%d", i+1) {
				return io.NopCloser(strings.NewReader("[" + strings.Join(page, ",") + "]")), nil
			}
		}
		if u == fmt.Sprintf("https://api.github.com/repos/me/app/releases?per_page=100&page=%d", len(pages)+1) {
			return io.NopCloser(strings.NewReader("[]")), nil
		}
		return nil, fmt.Errorf("GET %s", u)
	}
	rel := func(tag string, draft, pre bool) string {
		return fmt.Sprintf(`{"tag_name": %q, "draft": %t, "prerelease": %t, "body": "not \"tag_name\": \"desktop-v9.0.0\""}`, tag, draft, pre)
	}

	pages = [][]string{{
		rel("cli-v2.0.0", false, false),
		rel("desktop-v1.3.0", true, false),
		rel("desktop-v1.2.0-beta.1", false, true),
		rel("desktop-v1.1.0", false, false),
		rel("desktop-v1.0.0", false, false),
	}}
	got, err := ResolveFeed(context.Background(), feed, get)
	if want := "https://github.com/me/app/releases/download/desktop-v1.1.0/update-linux-amd64.json"; err != nil || got != want {
		t.Errorf("ResolveFeed = %s, %v, want %s", got, err, want)
	}

	// A full page leads to the next.
	var full []string
	for i := range releasesPerPage {
		full = append(full, rel(fmt.Sprintf("cli-v1.0.%d", i), false, false))
	}
	pages = [][]string{full, {rel("desktop-v0.9.0", false, false)}}
	if got, err := ResolveFeed(context.Background(), feed, get); err != nil || !strings.Contains(got, "/desktop-v0.9.0/") {
		t.Errorf("ResolveFeed = %s, %v", got, err)
	}

	pages = [][]string{{rel("cli-v2.0.0", false, false)}}
	if _, err := ResolveFeed(context.Background(), feed, get); !errors.Is(err, ErrNotPublished) {
		t.Errorf("without a release, err = %v", err)
	}

	// Other feeds are the manifest.
	plain := "https://github.com/me/app/releases/latest/download/update-linux-amd64.json"
	if got, err := ResolveFeed(context.Background(), plain, nil); err != nil || got != plain {
		t.Errorf("ResolveFeed = %s, %v", got, err)
	}
	if _, err := ResolveFeed(context.Background(), "https://example.com/{version}/update.json", get); err == nil {
		t.Error("a feed with {version} outside GitHub releases was resolved")
	}
}
