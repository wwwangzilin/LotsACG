package repo

import (
	"context"

	"github.com/krau/LotsACG/internal/model/entity"
	"github.com/krau/LotsACG/internal/shared"
	"github.com/unvgo/ouid"
)

type Video interface {
	UpdateVideoTelegramInfoByID(ctx context.Context, id ouid.OUID, tgInfo *shared.TelegramInfo) (*entity.Video, error)
}
