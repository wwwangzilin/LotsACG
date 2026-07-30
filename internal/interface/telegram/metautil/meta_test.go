package metautil

import (
	"testing"

	"github.com/wwwangzilin/LotsACG/internal/model/entity"
	"github.com/mymmrac/telego"
)

func TestResolvePostChatIDUsesR18ChannelForR18Artwork(t *testing.T) {
	primary := telegoutil.ID(100)
	r18 := telegoutil.ID(200)
	meta := NewMetaData(primary, "bot", WithR18ChannelChatID(r18))

	artwork := &entity.CachedArtworkData{CachedArtwork: entity.CachedArtwork{R18: true}}
	got := meta.ResolvePostChatID(artwork)
	if got.ID != r18.ID {
		t.Fatalf("expected r18 channel %d, got %d", r18.ID, got.ID)
	}
}

func TestResolvePostChatIDUsesR18ChannelForR18Tags(t *testing.T) {
	primary := telegoutil.ID(100)
	r18 := telegoutil.ID(200)
	meta := NewMetaData(primary, "bot", WithR18ChannelChatID(r18))

	artwork := &entity.CachedArtworkData{CachedArtwork: entity.CachedArtwork{Tags: []string{"normal", "R-18"}}}
	got := meta.ResolvePostChatID(artwork)
	if got.ID != r18.ID {
		t.Fatalf("expected r18 channel %d for tag-based r18 artwork, got %d", r18.ID, got.ID)
	}
}

func TestResolvePostChatIDFallsBackToPrimaryChannel(t *testing.T) {
	primary := telegoutil.ID(100)
	r18 := telegoutil.ID(200)
	meta := NewMetaData(primary, "bot", WithR18ChannelChatID(r18))

	artwork := &entity.CachedArtworkData{CachedArtwork: entity.CachedArtwork{R18: false, Tags: []string{"normal"}}}
	got := meta.ResolvePostChatID(artwork)
	if got.ID != primary.ID {
		t.Fatalf("expected primary channel %d, got %d", primary.ID, got.ID)
	}
}
