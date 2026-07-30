package repo

import (
	"context"

	"github.com/krau/LotsACG/internal/model/entity"
	"github.com/krau/LotsACG/internal/shared"
	"github.com/unvgo/ouid"
)

type Artist interface {
	GetArtistByID(ctx context.Context, id ouid.OUID) (*entity.Artist, error)
	GetArtistByUID(ctx context.Context, uid string, sourceType shared.SourceType) (*entity.Artist, error)
	UpdateArtist(ctx context.Context, patch *entity.Artist) error
	CreateArtist(ctx context.Context, artist *entity.Artist) (*ouid.OUID, error)
}
