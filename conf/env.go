package conf

import (
	"encoding/json"
	"fmt"
	"os"
)

var (
	Config  *configJSON
	Feature *FeatureStatus
)

// FeatureStatus 供前端判断翻译 / LLM 是否已按当前 Provider 配齐可用凭据。
type FeatureStatus struct {
	Trans bool   `json:"trans"`
	LLM   bool   `json:"llm"`
	SN    string `json:"sn"`
}

// configJSON 应用 JSON 配置根结构；百度翻译字段与 PHP env 同名，便于共用配置文件片段。
type configJSON struct {
	StudioServerPort              string `json:"STUDIO_SERVER_PORT"`
	StudioAutoOpen                bool   `json:"STUDIO_AUTO_OPEN"`
	StudioPath                    string `json:"STUDIO_PATH"`
	GeneratorPath                 string `json:"GENERATOR_PATH"`
	ProjectName                   string `json:"PROJECT_NAME"`
	ProjectDir                    string `json:"PROJECT_DIR"`
	TemplatesPath                 string `json:"TEMPLATES_PATH"`
	DataPath                      string `json:"DATA_PATH"`
	RuntimePath                   string `json:"RUNTIME_PATH"`
	LogPath                       string `json:"LOG_PATH"`
	PromptPath                    string `json:"PROMPT_PATH"`
	TransAPIKey                   string `json:"TRANS_API_KEY"`
	TransAPISecret                string `json:"TRANS_API_SECRET"`
	TransProvider                 string `json:"TRANS_PROVIDER"`
	TransProxy                    string `json:"TRANS_PROXY"`
	TransBaiduAPIURL              string `json:"TRANS_BAIDU_API_URL"`
	TransAliyunAPIURL             string `json:"TRANS_ALIYUN_API_URL"`
	TransTencentAPIHost           string `json:"TRANS_TENCENT_API_HOST"`
	TransTencentAPIVersion        string `json:"TRANS_TENCENT_API_VERSION"`
	TransTencentAPIAction         string `json:"TRANS_TENCENT_API_ACTION"`
	TransTencentAPIRegion         string `json:"TRANS_TENCENT_API_REGION"`
	LLMAPIKey                     string `json:"LLM_API_KEY"`
	LLMProvider                   string `json:"LLM_PROVIDER"`
	LLMProxy                      string `json:"LLM_PROXY"`
	LLMDeepseekChatCompletionsURL string `json:"LLM_DEEPSEEK_CHAT_COMPLETIONS_URL"`
	LLMDeepseekModelID            string `json:"LLM_DEEPSEEK_MODEL_ID"`
	LLMDoubaoChatCompletionsURL   string `json:"LLM_DOUBAO_CHAT_COMPLETIONS_URL"`
	LLMDoubaoModelID              string `json:"LLM_DOUBAO_MODEL_ID"`
	LLMQwenChatCompletionsURL     string `json:"LLM_QWEN_CHAT_COMPLETIONS_URL"`
	LLMQwenModelID                string `json:"LLM_QWEN_MODEL_ID"`
	LLMGLMChatCompletionsURL      string `json:"LLM_GLM_CHAT_COMPLETIONS_URL"`
	LLMGLMModelID                 string `json:"LLM_GLM_MODEL_ID"`
	LLMOpenaiChatCompletionsURL   string `json:"LLM_OPENAI_CHAT_COMPLETIONS_URL"`
	LLMOpenaiModelID              string `json:"LLM_OPENAI_MODEL_ID"`
	LLMClaudeChatCompletionsURL   string `json:"LLM_CLAUDE_CHAT_COMPLETIONS_URL"`
	LLMClaudeModelID              string `json:"LLM_CLAUDE_MODEL_ID"`
	LLMClaudeVersion              string `json:"LLM_CLAUDE_VERSION"`
	LLMClaudeMaxTokens            int    `json:"LLM_CLAUDE_MAX_TOKENS"`
	ProSN                         string `json:"PRO_SN"`
}

func LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &Config); err != nil {
		return fmt.Errorf("解析 JSON: %w", err)
	}

	return nil
}

// GetFeatureStatus 根据 Env 计算 trans、llm 是否视为已开启（与 studio 路由中的校验规则一致）。
func InitFeatureStatus() {
	Feature = &FeatureStatus{
		Trans: Config.TransAPIKey != "" && Config.TransAPISecret != "",
		LLM:   Config.LLMAPIKey != "",
		SN:    Config.ProSN,
	}
	fmt.Println("Use LLM: ", Feature.LLM)
	fmt.Println("Use Trans: ", Feature.Trans)
	fmt.Println("Use SN: ", Feature.SN != "")
}
