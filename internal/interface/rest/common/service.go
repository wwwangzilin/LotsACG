package common

import (
	"context"

	"github.com/gofiber/fiber/v3"
)

const (
	StateKeyService     = "serv"
	StateKeyConfig      = "cfg"
	StateKeyLogger      = "logger"
	StateKeyTelegramBot = "telegrambot" // DO NOT USE MustGetState to get this value!
)

type TelegramBot interface {
	SendArtworkInfo(ctx context.Context, sourceUrl string, chatID int64, appendCaption string)
	// PostArtworkToChannel 将指定来源链接的作品发布到主频道 (供 XP-Pusher 等外部调用)。
	PostArtworkToChannel(ctx context.Context, sourceURL string) error
}

func GetState[T any](ctx fiber.Ctx, key string) (T, bool) {
	val, ok := ctx.App().State().Get(key)
	if !ok {
		var zero T
		return zero, false
	}
	vv, ok := val.(T)
	if !ok {
		var zero T
		return zero, false
	}
	return vv, true
}

func MustGetState[T any](ctx fiber.Ctx, key string) T {
	val := ctx.App().State().MustGet(key)
	v, ok := val.(T)
	if !ok {
		panic("state: dependency type assertion failed!")
	}
	return v
}
