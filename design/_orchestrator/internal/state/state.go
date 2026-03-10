package state

type TokenUsage struct {
	InputTokens       int `json:"input_tokens"`
	CachedInputTokens int `json:"cached_input_tokens"`
	OutputTokens      int `json:"output_tokens"`
}

func (t *TokenUsage) Add(other TokenUsage) {
	t.InputTokens += other.InputTokens
	t.CachedInputTokens += other.CachedInputTokens
	t.OutputTokens += other.OutputTokens
}
