package handlers

import "testing"

func TestRecommendationSessionAddLike(t *testing.T) {
	session := &recommendationSession{}
	session.addLike("https://example.com/a")
	session.addLike("https://example.com/a")
	session.addLike("https://example.com/b")

	if len(session.LikedSourceURLs) != 2 {
		t.Fatalf("expected 2 liked items, got %d", len(session.LikedSourceURLs))
	}
	if session.LikedSourceURLs[0] != "https://example.com/a" {
		t.Fatalf("unexpected first liked item: %s", session.LikedSourceURLs[0])
	}
	if session.LikedSourceURLs[1] != "https://example.com/b" {
		t.Fatalf("unexpected second liked item: %s", session.LikedSourceURLs[1])
	}
}

func TestIsSeenSourceURL(t *testing.T) {
	session := &recommendationSession{SeenSourceURLs: []string{"https://example.com/seen"}}
	if !isSeenSourceURL(session, "https://example.com/seen") {
		t.Fatal("expected seen source url to be detected")
	}
	if isSeenSourceURL(session, "https://example.com/other") {
		t.Fatal("did not expect unseen source url to be detected")
	}
}
