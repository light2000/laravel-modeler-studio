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

type glmThinkingObj struct {
	Type string `json:"type"`
}

type glmChatCompletionRequest struct {
	Model       string          `json:"model"`
	Messages    []Message       `json:"messages"`
	Thinking    *glmThinkingObj `json:"thinking,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

// GlmTableAISuggestAttrs 调用智谱 AI 开放平台（BigModel）[对话补全]，解析为表字段推荐结果。
// apiKey 为空则返回错误。
// 请求示例：
//
//	curl -X POST "https://open.bigmodel.cn/api/paas/v4/chat/completions" \
//	  -H "Content-Type: application/json" \
//	  -H "Authorization: Bearer your-api-key" \
//	  -d '{"model":"glm-5","messages":[{"role":"user","content":"..." }],"thinking":{"type":"enabled"},"max_tokens":65536,"temperature":1.0}'
func GlmTableAISuggestAttrs(glmChatCompletionsURL string, glmModelChat string, apiKey string, project *proto.Project, request *SuggestAttrsAIRequest) (*TableAISuggestAttrsData, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("glm: 缺少 API Key")
	}

	messages, err := TableAISuggestAttrsPrompt(project, request)
	if err != nil {
		return nil, err
	}

	// BigModel v4 兼容 chat/completions，并支持 thinking/max_tokens/temperature 等扩展字段。
	body, err := json.Marshal(glmChatCompletionRequest{
		Model:    glmModelChat,
		Messages: messages,
		Thinking: &glmThinkingObj{
			Type: "enabled",
		},
		MaxTokens:   65536,
		Temperature: 1.0,
	})
	if err != nil {
		logger.Errorf("glm: 序列化请求: %v", err)
		return nil, fmt.Errorf("glm: 序列化请求: %w", err)
	}

	logger.Infof("glm: 调用 GLM 表字段推荐: %s", string(body))

	req, err := http.NewRequest(http.MethodPost, glmChatCompletionsURL, bytes.NewReader(body))
	if err != nil {
		logger.Errorf("glm: 构造请求: %v", err)
		return nil, fmt.Errorf("glm: 构造请求: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))

	resp, err := HTTPClient.Do(req)
	if err != nil {
		logger.Errorf("glm: 请求失败: %v", err)
		return nil, fmt.Errorf("glm: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("glm: 读取响应: %v", err)
		return nil, fmt.Errorf("glm: 读取响应: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		logger.Errorf("glm: HTTP %s: %s", resp.Status, truncateForErr(raw, 512))
		return nil, fmt.Errorf("glm: HTTP %s: %s", resp.Status, truncateForErr(raw, 512))
	}

	var apiResp chatCompletionResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		logger.Errorf("glm: 解析响应 JSON: %v", err)
		return nil, fmt.Errorf("glm: 解析响应 JSON: %w", err)
	}
	if len(apiResp.Choices) == 0 {
		logger.Errorf("glm: 响应缺少 choices")
		return nil, fmt.Errorf("glm: 响应缺少 choices")
	}
	if apiResp.Choices[0].FinishReason == "length" {
		logger.Errorf("glm: 输出因长度被截断 (finish_reason=length)")
		return nil, fmt.Errorf("glm: 输出因长度被截断 (finish_reason=length)")
	}

	content := strings.TrimSpace(apiResp.Choices[0].Message.Content)
	if content == "" {
		logger.Errorf("glm: 模型返回空 content")
		return nil, fmt.Errorf("glm: 模型返回空 content")
	}

	content = extractJSONObject(content)
	logger.Infof("glm: 表字段推荐结果: %s", content)

	var data TableAISuggestAttrsData
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		logger.Errorf("glm: 解析业务 JSON: %v", err)
		return nil, fmt.Errorf("glm: 解析业务 JSON: %w", err)
	}

	return &data, nil
}
