// Package aiapi 提供一个 OpenAI 兼容的聊天补全客户端,
// 用于推荐系统中根据用户偏好 tag 自动生成关联的相似 tag。
package aiapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/imroc/req/v3"
	"github.com/samber/oops"
	config "github.com/wwwangzilin/LotsACG/internal/infra/config/runtimecfg"
	"github.com/wwwangzilin/LotsACG/pkg/log"
)

// Client 是 OpenAI 兼容的 AI API 客户端。
type Client struct {
	cfg       config.AIAPIConfig
	reqClient *req.Client
}

// New 创建 AI API 客户端。优先使用 [aiapi] 配置, 若未启用则回退到 [xpaiapi] 配置。
func New(aiapiCfg config.AIAPIConfig, xpCfg config.XPAIAPIConfig) *Client {
	cfg := aiapiCfg
	if !cfg.Enable {
		// 回退到 xpaiapi 配置
		cfg.Enable = xpCfg.Enabled
		if cfg.BaseURL == "" {
			cfg.BaseURL = xpCfg.BaseURL
		}
		if cfg.APIKey == "" {
			cfg.APIKey = xpCfg.APIKey
		}
		if cfg.Model == "" {
			cfg.Model = xpCfg.Model
		}
	}
	c := req.C().
		SetLogger(log.Default()).
		SetTimeout(30 * time.Second).
		SetCommonRetryCount(2)
	if cfg.BaseURL != "" {
		c = c.SetBaseURL(cfg.BaseURL)
	}
	if cfg.APIKey != "" {
		c = c.SetCommonHeader("Authorization", "Bearer "+cfg.APIKey)
	}
	return &Client{
		cfg:       cfg,
		reqClient: c,
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.cfg.Enable && c.cfg.BaseURL != ""
}

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []chatCompletionMessage `json:"messages"`
	Temperature float64                 `json:"temperature"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatCompletionMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ExpandTags 根据给定的用户偏好 tags 生成 count 个相关联的 Pixiv 搜索 tag。
// 返回按重要性排序的 tag 列表; 若 AI 不可用或失败, 返回空切片(调用方应回退到原始 tags)。
func (c *Client) ExpandTags(ctx context.Context, tags []string, count int) ([]string, error) {
	if !c.Enabled() {
		return nil, nil
	}
	if count <= 0 {
		count = 12
	}
	tagList := strings.Join(tags, ", ")
	prompt := fmt.Sprintf(`你是一名动漫插画(Pixiv)标签专家。以下是一位用户的偏好标签列表:
%s

请根据这些偏好标签, 生成 %d 个与之关联或相似的 Pixiv 搜索标签, 用于在 Pixiv 上搜索用户可能喜欢的新作品。
要求:
- 标签可以是同义词、常见搭配、风格、角色属性等
- 尽量使用 Pixiv 上常见的日文或英文标签, 也可以保留中文
- 不要包含 R-18 相关标签
- 只输出标签列表, 每个标签一行, 不要编号, 不要解释

生成的标签:`, tagList, count)

	req := chatCompletionRequest{
		Model: c.cfg.Model,
		Messages: []chatCompletionMessage{
			{Role: "system", Content: "你是一个帮助生成动漫插画搜索标签的助手, 只输出标签列表, 不要输出其他内容。"},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.9,
	}

	var resp chatCompletionResponse
	httpResp, err := c.reqClient.R().
		SetContext(ctx).
		SetBody(req).
		SetSuccessResult(&resp).
		Post("/chat/completions")
	if err != nil {
		return nil, oops.Wrapf(err, "ai api request failed")
	}
	if httpResp.IsErrorState() {
		if resp.Error != nil {
			return nil, oops.Errorf("ai api error: %s", resp.Error.Message)
		}
		return nil, oops.Errorf("ai api http error: %d", httpResp.GetStatusCode())
	}
	if len(resp.Choices) == 0 {
		return nil, oops.New("ai api returned no choices")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" {
		return nil, oops.New("ai api returned empty content")
	}
	return parseTagList(content), nil
}

// parseTagList 解析 AI 返回的标签列表。兼容逗号分隔、换行分隔、以及带编号/引号/方括号的格式。
func parseTagList(content string) []string {
	// 去除可能的 markdown 代码块
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```text")
	content = strings.TrimPrefix(content, "```plaintext")
	content = strings.Trim(content, "`")

	// 尝试 JSON 数组
	if strings.HasPrefix(content, "[") && strings.HasSuffix(content, "]") {
		var arr []string
		if err := json.Unmarshal([]byte(content), &arr); err == nil {
			return cleanTags(arr)
		}
	}

	// 按换行或逗号分隔
	replacer := strings.NewReplacer("\r", "", "\n", ",", "，", ",", "、", ",")
	content = replacer.Replace(content)
	parts := strings.Split(content, ",")

	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"'*。-–—:：`)
		// 去掉 "1. " 之类的编号 (使用 rune 索引以安全处理多字节字符)
		runes := []rune(p)
		if len(runes) > 2 && (runes[1] == '.' || runes[1] == '、' || runes[1] == ')') {
			if runes[0] >= '0' && runes[0] <= '9' {
				p = string(runes[2:])
			}
		}
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		tags = append(tags, p)
	}
	return cleanTags(tags)
}

func cleanTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}
