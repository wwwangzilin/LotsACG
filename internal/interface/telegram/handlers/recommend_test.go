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

func TestRecommendationSessionAddSeen(t *testing.T) {
	session := &recommendationSession{}
	session.addSeen("https://example.com/a")
	session.addSeen("https://example.com/a")
	session.addSeen("https://example.com/b")

	if len(session.SeenSourceURLs) != 2 {
		t.Fatalf("expected 2 seen items, got %d", len(session.SeenSourceURLs))
	}
	if session.SeenSourceURLs[0] != "https://example.com/a" {
		t.Fatalf("unexpected first seen item: %s", session.SeenSourceURLs[0])
	}
	if session.SeenSourceURLs[1] != "https://example.com/b" {
		t.Fatalf("unexpected second seen item: %s", session.SeenSourceURLs[1])
	}
}

func TestRecommendationSessionSeenCap(t *testing.T) {
	session := &recommendationSession{}
	for i := 0; i < 25; i++ {
		session.addSeen("https://example.com/")
	}
	if len(session.SeenSourceURLs) > 20 {
		t.Fatalf("seen urls exceeded cap of 20, got %d", len(session.SeenSourceURLs))
	}
}
