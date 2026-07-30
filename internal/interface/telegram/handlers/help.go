package handlers

import (
	"fmt"

	"github.com/wwwangzilin/LotsACG/internal/common/version"
	"github.com/wwwangzilin/LotsACG/internal/interface/telegram/handlers/utils"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
)

func Help(ctx *telegohandler.Context, message telego.Message) error {
	serv, err := requireService(ctx)
	if err != nil {
		return err
	}
	helpText := `使用方法:
/setu - 随机图片(NSFW)
/random - 随机全年龄图�?
/search - 搜索相似图片
/info - 发送作品图片和信息
/files - 获取作品原图
/hybrid - 混合搜索作品
/similar - 搜索相似作品
/recommend - 私聊中智能推荐作�?基于你喜�?不喜欢的标签偏好)
`
	helpText += `
随机图片相关功能中支持使用以下格式的参数:
使用 '|' 分隔'�?关系, 使用 '空格' 分隔'�?关系, 示例:

/random 萝莉|白丝 猫耳|原创

表示搜索包含"萝莉"�?白丝", 且包�?猫�?�?原创"的图�?
Inline 查询(在任意聊天框中@本bot)支持同样的参数格�?
`
	isAdmin, _ := serv.IsAdminByTgID(ctx, message.From.ID)
	if isAdmin {
		helpText += `
管理员命�?
/addadmin - 添加管理�?
/deladmin - 删除管理�?
/delete - 删除整个作品
/r18 - 设置作品R18标记
/title - 设置作品标题
/tags - 更新作品标签(覆盖原有标签)
/autotag - 自动tag作品
/addtags - 添加作品标签
/deltags - 删除作品标签
/tagalias - 为标签添加别�?
/dump - 输出 json 格式作品信息
/recaption - 重新生成作品描述
/reindex - 重新索引作品
`
	}
	helpText += fmt.Sprintf("\n版本: %s, 构建日期 %s, 提交 %s\nhttps://github.com/wwwangzilin/LotsACG", version.Version, version.BuildTime, version.Commit[:7])
	utils.ReplyMessage(ctx, message, helpText)
	return nil
}
