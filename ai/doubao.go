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

// DouBaoTableAISuggestAttrs 调用豆包（火山方舟）[对话补全]（JSON 模式），解析为表字段推荐结果。
// apiKey 为 DOUBAO_API_KEY；为空则返回错误。
func DouBaoTableAISuggestAttrs(doubaoChatCompletionsURL string, doubaoModelChat string, apiKey string, project *proto.Project, request *SuggestAttrsAIRequest) (*TableAISuggestAttrsData, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("doubao: 缺少 API Key")
	}

	messages, err := TableAISuggestAttrsPrompt(project, request)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(chatCompletionRequest{
		Model:    doubaoModelChat,
		Messages: messages,
	})
	if err != nil {
		logger.Errorf("doubao: 序列化请求: %v", err)
		return nil, fmt.Errorf("doubao: 序列化请求: %w", err)
	}

	logger.Infof("doubao: 调用豆包表字段推荐: %s", string(body))

	req, err := http.NewRequest(http.MethodPost, doubaoChatCompletionsURL, bytes.NewReader(body))
	if err != nil {
		logger.Errorf("doubao: 构造请求: %v", err)
		return nil, fmt.Errorf("doubao: 构造请求: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))

	resp, err := HTTPClient.Do(req)
	if err != nil {
		logger.Errorf("doubao: 请求失败: %v", err)
		return nil, fmt.Errorf("doubao: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("doubao: 读取响应: %v", err)
		return nil, fmt.Errorf("doubao: 读取响应: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("doubao: HTTP %s: %s", resp.Status, truncateForErr(raw, 512))
		return nil, fmt.Errorf("doubao: HTTP %s: %s", resp.Status, truncateForErr(raw, 512))
	}

	var apiResp chatCompletionResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		logger.Errorf("doubao: 解析响应 JSON: %v", err)
		return nil, fmt.Errorf("doubao: 解析响应 JSON: %w", err)
	}
	if len(apiResp.Choices) == 0 {
		logger.Errorf("doubao: 响应缺少 choices")
		return nil, fmt.Errorf("doubao: 响应缺少 choices")
	}
	if apiResp.Choices[0].FinishReason == "length" {
		logger.Errorf("doubao: 输出因长度被截断 (finish_reason=length)")
		return nil, fmt.Errorf("doubao: 输出因长度被截断 (finish_reason=length)")
	}

	content := strings.TrimSpace(apiResp.Choices[0].Message.Content)
	if content == "" {
		logger.Errorf("doubao: 模型返回空 content")
		return nil, fmt.Errorf("doubao: 模型返回空 content")
	}
	content = extractJSONObject(content)
	logger.Infof("doubao: 表字段推荐结果: %s", content)

	var data TableAISuggestAttrsData
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		logger.Errorf("doubao: 解析业务 JSON: %v", err)
		return nil, fmt.Errorf("doubao: 解析业务 JSON: %w", err)
	}

	return &data, nil
}
