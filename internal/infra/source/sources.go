package source

import (
	"context"

	"github.com/wwwangzilin/LotsACG/internal/model/dto"
	"github.com/wwwangzilin/LotsACG/internal/shared"
)

type ArtworkSource interface {
	GetArtworkInfo(ctx context.Context, sourceUrl string) (*dto.FetchedArtwork, error)
	MatchesSourceURL(sourceUrl string) (string, bool)
	FetchNewArtworks(ctx context.Context, limit int) ([]*dto.FetchedArtwork, error)
	PrettyFileName(artwork shared.ArtworkLike, picture shared.PictureLike) string
}
