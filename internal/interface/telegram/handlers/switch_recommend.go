package handlers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
	"github.com/wwwangzilin/LotsACG/internal/interface/telegram/handlers/utils"
	"github.com/wwwangzilin/LotsACG/internal/service"
)

// SwitchRecommendSource 处理 /switchrecommend 指令:
// 切换推荐使用的图源。不带参数时列出当前可用图源。
func SwitchRecommendSource(ctx *telegohandler.Context, message telego.Message) error {
	serv, err := requireService(ctx)
	if err != nil {
		return err
	}
	_, _, args := telegoutil.ParseCommand(message.Text)
	arg := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))

	// 可用图源 (来自 service 的已启用源)
	sources := serv.Sources()
	names := make([]string, 0, len(sources))
	for st := range sources {
		names = append(names, string(st))
	}
	sort.Strings(names)
	if len(names) == 0 {
		utils.ReplyMessage(ctx, message, "当前没有可用的图源")
		return nil
	}

	current := service.GetUserRecommendSource(ctx, message.From.ID)
	if current == "" {
		current = "all"
	}

	if arg == "" {
		var sb strings.Builder
		sb.WriteString("当前推荐图源: <code>")
		sb.WriteString(utils.EscapeHTML(current))
		sb.WriteString("</code>\n\n可选图源:\n")
		for _, n := range names {
			sb.WriteString("· <code>")
			sb.WriteString(utils.EscapeHTML(n))
			sb.WriteString("</code>\n")
		}
		sb.WriteString("· <code>all</code> (全部)\n\n用法: /switchrecommend <图源>")
		utils.ReplyMessage(ctx, message, sb.String())
		return nil
	}

	if arg == "all" {
		if err := service.SetUserRecommendSource(ctx, message.From.ID, ""); err != nil {
			utils.ReplyMessage(ctx, message, "设置失败, 请稍后再试")
			return err
		}
		utils.ReplyMessage(ctx, message, "推荐图源已切换为 <code>all</code> (全部)")
		return nil
	}

	valid := false
	for _, n := range names {
		if n == arg {
			valid = true
			break
		}
	}
	if !valid {
		utils.ReplyMessage(ctx, message, fmt.Sprintf("无效的图源 <code>%s</code>, 可选: %s, all", utils.EscapeHTML(arg), strings.Join(names, ", ")))
		return nil
	}
	if err := service.SetUserRecommendSource(ctx, message.From.ID, arg); err != nil {
		utils.ReplyMessage(ctx, message, "设置失败, 请稍后再试")
		return err
	}
	utils.ReplyMessage(ctx, message, "推荐图源已切换为 <code>"+utils.EscapeHTML(arg)+"</code>")
	return nil
}
