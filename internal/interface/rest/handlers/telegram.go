package handlers

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/wwwangzilin/LotsACG/internal/interface/rest/common"
	"github.com/wwwangzilin/LotsACG/internal/service"
	"github.com/wwwangzilin/LotsACG/internal/shared"
)

type RequestSendArtworkInfoByTelegramBot struct {
	SourceURL     string `json:"source_url" query:"source_url" form:"source_url" validate:"required"`
	ChatID        int64  `json:"chat_id" query:"chat_id" form:"chat_id" validate:"required"`
	AppendCaption string `json:"append_caption" query:"append_caption" form:"append_caption"`
}

func HandleSendArtworkInfoByTelegramBot(ctx fiber.Ctx) error {
	requestCtx := ctx.RequestCtx()
	key := ctx.Get("X-API-KEY")
	if key == "" {
		return common.NewError(fiber.StatusUnauthorized, "api key is required")
	}
	serv := common.MustGetState[*service.Service](ctx, common.StateKeyService)
	keyEnt, err := serv.GetApiKeyByKey(requestCtx, key)
	if err != nil {
		return common.NewError(fiber.StatusUnauthorized, "invalid api key")
	}
	if !keyEnt.HasPermission(shared.PermissionSendArtworkInfo) {
		return common.NewError(fiber.StatusForbidden, "api key does not have permission")
	}
	if !keyEnt.CanUse() {
		return common.NewError(fiber.StatusForbidden, "api key quota exceeded")
	}
	bot, ok := common.GetState[common.TelegramBot](ctx, common.StateKeyTelegramBot)
	if !ok {
		return fiber.ErrInternalServerError
	}
	req := new(RequestSendArtworkInfoByTelegramBot)
	if err := ctx.Bind().All(req); err != nil {
		return err
	}
	serv.IncreaseApiKeyUsed(requestCtx, key)
	// current implement of SendArtworkInfo use a buffered channel, so it will return immediately and run in the background.
	// thus we should use context.Background() here.
	go bot.SendArtworkInfo(context.Background(), req.SourceURL, req.ChatID, req.AppendCaption)
	return ctx.JSON(common.NewSuccess("ok"))
}

// RequestPostArtworkToChannel 由外部 (如 XP-Pusher) 请求将作品发布到主频道。
type RequestPostArtworkToChannel struct {
	SourceURL string `json:"source_url" query:"source_url" form:"source_url" validate:"required"`
}

// HandlePostArtworkToChannel 将指定来源链接的作品发布到主频道。
// 供 XP-Pusher 的「推送到群」按钮调用。
func HandlePostArtworkToChannel(ctx fiber.Ctx) error {
	requestCtx := ctx.RequestCtx()
	key := ctx.Get("X-API-KEY")
	if key == "" {
		return common.NewError(fiber.StatusUnauthorized, "api key is required")
	}
	serv := common.MustGetState[*service.Service](ctx, common.StateKeyService)
	keyEnt, err := serv.GetApiKeyByKey(requestCtx, key)
	if err != nil {
		return common.NewError(fiber.StatusUnauthorized, "invalid api key")
	}
	if !keyEnt.HasPermission(shared.PermissionPostArtwork) {
		return common.NewError(fiber.StatusForbidden, "api key does not have permission")
	}
	if !keyEnt.CanUse() {
		return common.NewError(fiber.StatusForbidden, "api key quota exceeded")
	}
	bot, ok := common.GetState[common.TelegramBot](ctx, common.StateKeyTelegramBot)
	if !ok {
		return fiber.ErrInternalServerError
	}
	req := new(RequestPostArtworkToChannel)
	if err := ctx.Bind().All(req); err != nil {
		return err
	}
	serv.IncreaseApiKeyUsed(requestCtx, key)
	if err := bot.PostArtworkToChannel(context.Background(), req.SourceURL); err != nil {
		return common.NewError(fiber.StatusInternalServerError, "post artwork to channel failed: "+err.Error())
	}
	return ctx.JSON(common.NewSuccess("ok"))
}
