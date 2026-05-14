package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/light2000/laravel-modeler-studio/logger"
	"github.com/light2000/laravel-modeler-studio/proto"
)

// Anthropic Messages API（与 OpenAI chat/completions 不同）：文档见
// https://platform.claude.com/docs/en/api/overview

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeMessagesRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
}

func splitPromptForClaude(openAIMessages []Message) (system string, messages []claudeMessage, err error) {
	var sysParts []string
	for _, m := range openAIMessages {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		switch role {
		case "system":
			sysParts = append(sysParts, m.Content)
		case "user", "assistant":
			messages = append(messages, claudeMessage{Role: role, Content: m.Content})
		default:
			return "", nil, fmt.Errorf("claude: 不支持的消息角色: %q", m.Role)
		}
	}
	system = strings.TrimSpace(strings.Join(sysParts, "\n\n"))
	if len(messages) == 0 {
		return "", nil, fmt.Errorf("claude: messages 不能为空（至少一条 user/assistant）")
	}
	return system, messages, nil
}

func claudeResponseText(resp *claudeMessagesResponse) string {
	var b strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			b.WriteString(block.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

// ClaudeTableAISuggestAttrs 调用 Claude Messages API（POST /v1/messages），解析为表字段推荐结果。
// claudeMessagesURL 一般为 https://api.anthropic.com/v1/messages（或通过代理的完整 Messages 路径）。
// apiKey 对应 Console 中的 API key；anthropicVersion 写入请求头 anthropic-version（如 2023-06-01）。
// maxTokens 对应请求体 max_tokens；若 <= 0 则使用 8192。
func ClaudeTableAISuggestAttrs(claudeMessagesURL string, model string, apiKey string, anthropicVersion string, maxTokens int, project *proto.Project, request *SuggestAttrsAIRequest) (*TableAISuggestAttrsData, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("claude: 缺少 API Key")
	}
	url := strings.TrimSpace(claudeMessagesURL)
	if url == "" {
		url = "https://api.anthropic.com/v1/messages"
	}
	ver := strings.TrimSpace(anthropicVersion)
	if ver == "" {
		ver = "2023-06-01"
	}
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	openAIMessages, err := TableAISuggestAttrsPrompt(project, request)
	if err != nil {
		return nil, err
	}

	system, messages, err := splitPromptForClaude(openAIMessages)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(claudeMessagesRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  messages,
	})
	if err != nil {
		logger.Errorf("claude: 序列化请求: %v", err)
		return nil, fmt.Errorf("claude: 序列化请求: %w", err)
	}
	logger.Infof("claude: 调用 Claude 表字段推荐: %s", string(body))

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		logger.Errorf("claude: 构造请求: %v", err)
		return nil, fmt.Errorf("claude: 构造请求: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", strings.TrimSpace(apiKey))
	req.Header.Set("anthropic-version", ver)

	resp, err := HTTPClient.Do(req)
	if err != nil {
		logger.Errorf("claude: 请求失败: %v", err)
		return nil, fmt.Errorf("claude: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("claude: 读取响应: %v", err)
		return nil, fmt.Errorf("claude: 读取响应: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		logger.Errorf("claude: HTTP %s: %s", resp.Status, truncateForErr(raw, 512))
		return nil, fmt.Errorf("claude: HTTP %s: %s", resp.Status, truncateForErr(raw, 512))
	}

	var apiResp claudeMessagesResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		logger.Errorf("claude: 解析响应 JSON: %v", err)
		return nil, fmt.Errorf("claude: 解析响应 JSON: %w", err)
	}

	if apiResp.StopReason == "max_tokens" {
		logger.Errorf("claude: 输出因长度被截断 (stop_reason=max_tokens)")
		return nil, fmt.Errorf("claude: 输出因长度被截断 (stop_reason=max_tokens)")
	}

	content := claudeResponseText(&apiResp)
	if content == "" {
		logger.Errorf("claude: 模型返回空文本")
		return nil, fmt.Errorf("claude: 模型返回空文本")
	}

	content = extractJSONObject(content)
	logger.Infof("claude: 表字段推荐结果: %s", content)

	var data TableAISuggestAttrsData
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		logger.Errorf("claude: 解析业务 JSON: %v", err)
		return nil, fmt.Errorf("claude: 解析业务 JSON: %w", err)
	}

	return &data, nil
}
