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

// ArtworkTagSearcher 可选接口: 支持按 tag 搜索新作品的源实现它。
// 通过类型断言使用, 不影响其他源。
type ArtworkTagSearcher interface {
	SearchArtworksByTags(ctx context.Context, tags []string, limit int) ([]*dto.FetchedArtwork, error)
}

// ArtworkTagSearcherOrdered 可选接口: 支持按 tag 搜索且可指定排序的源。
// 实现它的源可获得更精细的推荐排序 (热门/最新)。
type ArtworkTagSearcherOrdered interface {
	SearchArtworksByTagsOrdered(ctx context.Context, tags []string, limit int, order string) ([]*dto.FetchedArtwork, error)
}
