package repo

import (
	"context"

	"github.com/wwwangzilin/LotsACG/internal/model/entity"
	"github.com/wwwangzilin/LotsACG/internal/shared"
	"github.com/unvgo/ouid"
)

type Ugoira interface {
	UpdateUgoiraTelegramInfoByID(ctx context.Context, id ouid.OUID, tgInfo *shared.TelegramInfo) (*entity.UgoiraMeta, error)
}
