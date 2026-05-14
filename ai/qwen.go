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

// QwenTableAISuggestAttrs 调用通义千问（DashScope Compatible Mode）[对话补全]（JSON 模式），解析为表字段推荐结果。
// apiKey 为 DASHSCOPE_API_KEY；为空则返回错误。
// 请求示例：
// curl -X POST https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions \
// -H "Authorization: Bearer $DASHSCOPE_API_KEY" \
// -H "Content-Type: application/json" \
// -d '{"model":"qwen-plus","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"你是谁？"}]}'
func QwenTableAISuggestAttrs(qwenChatCompletionsURL string, qwenModelChat string, apiKey string, project *proto.Project, request *SuggestAttrsAIRequest) (*TableAISuggestAttrsData, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("qwen: 缺少 API Key")
	}

	messages, err := TableAISuggestAttrsPrompt(project, request)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(chatCompletionRequest{
		Model:    qwenModelChat,
		Messages: messages,
		ResponseFormat: &responseFormatObj{
			Type: "json_object",
		},
	})
	if err != nil {
		logger.Errorf("qwen: 序列化请求: %v", err)
		return nil, fmt.Errorf("qwen: 序列化请求: %w", err)
	}

	logger.Infof("qwen: 调用通义千问表字段推荐: %s", string(body))

	req, err := http.NewRequest(http.MethodPost, qwenChatCompletionsURL, bytes.NewReader(body))
	if err != nil {
		logger.Errorf("qwen: 构造请求: %v", err)
		return nil, fmt.Errorf("qwen: 构造请求: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))

	resp, err := HTTPClient.Do(req)
	if err != nil {
		logger.Errorf("qwen: 请求失败: %v", err)
		return nil, fmt.Errorf("qwen: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("qwen: 读取响应: %v", err)
		return nil, fmt.Errorf("qwen: 读取响应: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		logger.Errorf("qwen: HTTP %s: %s", resp.Status, truncateForErr(raw, 512))
		return nil, fmt.Errorf("qwen: HTTP %s: %s", resp.Status, truncateForErr(raw, 512))
	}

	var apiResp chatCompletionResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		logger.Errorf("qwen: 解析响应 JSON: %v", err)
		return nil, fmt.Errorf("qwen: 解析响应 JSON: %w", err)
	}
	if len(apiResp.Choices) == 0 {
		logger.Errorf("qwen: 响应缺少 choices")
		return nil, fmt.Errorf("qwen: 响应缺少 choices")
	}
	if apiResp.Choices[0].FinishReason == "length" {
		logger.Errorf("qwen: 输出因长度被截断 (finish_reason=length)")
		return nil, fmt.Errorf("qwen: 输出因长度被截断 (finish_reason=length)")
	}

	content := strings.TrimSpace(apiResp.Choices[0].Message.Content)
	if content == "" {
		logger.Errorf("qwen: 模型返回空 content")
		return nil, fmt.Errorf("qwen: 模型返回空 content")
	}

	content = extractJSONObject(content)
	logger.Infof("qwen: 表字段推荐结果: %s", content)

	var data TableAISuggestAttrsData
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		logger.Errorf("qwen: 解析业务 JSON: %v", err)
		return nil, fmt.Errorf("qwen: 解析业务 JSON: %w", err)
	}

	return &data, nil
}
