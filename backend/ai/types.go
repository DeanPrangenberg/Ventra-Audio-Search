package ai

type ChatReq struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type Message struct {
	Role    string `json:"role"` // system|user|assistant
	Content string `json:"content"`
}

type ollamaEmbedReq struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type ollamaEmbedResp struct {
	Model      string      `json:"model"`
	Embeddings [][]float64 `json:"embeddings"`
	Error      string      `json:"error,omitempty"`
}

type ChatResp struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}
