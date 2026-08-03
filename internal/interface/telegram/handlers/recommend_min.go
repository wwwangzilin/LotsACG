package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
	"github.com/wwwangzilin/LotsACG/internal/interface/telegram/handlers/utils"
	"github.com/wwwangzilin/LotsACG/internal/service"
)

// RecommendMinBookmarks 处理 /recommendmin 指令:
// 设置推荐作品的最小收藏数 (低于该收藏数的作品不会出现在推荐中)。0 = 不限制。
func RecommendMinBookmarks(ctx *telegohandler.Context, message telego.Message) error {
	_, _, args := telegoutil.ParseCommand(message.Text)
	arg := strings.TrimSpace(strings.Join(args, " "))

	if arg == "" {
		cur := service.GetUserMinBookmarks(ctx, message.From.ID)
		if cur <= 0 {
			utils.ReplyMessage(ctx, message, "当前推荐不限制最小收藏数\n\n用法: /recommendmin <收藏数>\n设置后, 推荐只会出现收藏数不低于该值的作品 (0 = 不限制)")
		} else {
			utils.ReplyMessage(ctx, message, fmt.Sprintf("当前推荐最小收藏数: <code>%d</code>\n\n用法: /recommendmin <收藏数> (0 = 不限制)", cur))
		}
		return nil
	}

	min, err := strconv.Atoi(arg)
	if err != nil || min < 0 {
		utils.ReplyMessage(ctx, message, "参数错误, 请输入一个非负整数, 如 /recommendmin 100 (0 = 不限制)")
		return nil
	}
	if err := service.SetUserMinBookmarks(ctx, message.From.ID, min); err != nil {
		utils.ReplyMessage(ctx, message, "设置失败, 请稍后再试")
		return err
	}
	if min == 0 {
		utils.ReplyMessage(ctx, message, "已取消最小收藏数限制")
	} else {
		utils.ReplyMessage(ctx, message, fmt.Sprintf("已设置推荐最小收藏数: <code>%d</code>", min))
	}
	return nil
}
