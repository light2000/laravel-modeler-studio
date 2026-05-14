package ai

type OpenAICompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type chatCompletionRequest struct {
	Model          string             `json:"model"`
	Messages       []Message          `json:"messages"`
	ResponseFormat *responseFormatObj `json:"response_format,omitempty"`
}

type responseFormatObj struct {
	Type string `json:"type"`
}

type chatCompletionResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// TableAISuggestAttrsData 与 system_ai_suggest_attrs.tpl 中约定的 AI 输出 JSON 对应。
type TableAISuggestAttrsData struct {
	Table  string                     `json:"table"`
	Fields []TableAISuggestAttrsField `json:"fields"`
}

// TableAISuggestAttrsField 表示建议的单个字段。
// Type 取值：ID, INT, DECIMAL, BOOL, EMAIL, URL, IP, TXT_PHRASE, TXT_SENTENCE, TXT_PARAGRAPH,
// TXT_DOCUMENT, DATE, TIME, DATETIME, YEAR, ENUM, SETS, IMAGE, IMAGES, VIDEO, VIDEOS,
// AUDIO, AUDIOS, FILE, FILES。
// Index 取值：NONE, NORMAL, UNIQUE。
type TableAISuggestAttrsField struct {
	Code      string                       `json:"code"`
	Type      string                       `json:"type"`
	Nullable  bool                         `json:"nullable"`
	Index     string                       `json:"index"`
	Name      string                       `json:"name"`
	Abilities []string                     `json:"abilities"`
	Behavior  string                       `json:"behavior"`
	Relation  *TableAISuggestAttrsRelation `json:"relation"`
	EnumRef   *TableAISuggestAttrsEnumRef  `json:"enum_ref"`
	FakeMin   string                       `json:"fake_min"`
	FakeMax   string                       `json:"fake_max"`
}

// TableAISuggestAttrsRelation 对应仅当存在关联语义字段时的 relation 对象。
type TableAISuggestAttrsRelation struct {
	TargetModel              string  `json:"target_model"`
	Type                     string  `json:"type"`
	Name                     string  `json:"name"`
	InverseType              string  `json:"inverse_type"`
	InverseName              string  `json:"inverse_name"`
	Confidence               float64 `json:"confidence"`
	MatchedFromCandidates    bool    `json:"matched_from_candidates"`
	RequiresUserConfirmation bool    `json:"requires_user_confirmation"`
}

// TableAISuggestAttrsEnumRef 对应 type 为 ENUM 时的 enum_ref 对象。
type TableAISuggestAttrsEnumRef struct {
	Mode                     string   `json:"mode"`
	Code                     string   `json:"code"`
	Name                     string   `json:"name"`
	Options                  []Option `json:"options"`
	Confidence               float64  `json:"confidence"`
	MatchedFromCandidates    bool     `json:"matched_from_candidates"`
	RequiresUserConfirmation bool     `json:"requires_user_confirmation"`
}

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SuggestAttrsAIRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ModuleName  string `json:"module_name"`
}
