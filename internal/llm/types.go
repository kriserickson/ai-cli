package llm

// Chat API request types
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Chat API response types
type ChatResponse struct {
	Choices []Choice  `json:"choices"`
	Error   *APIError `json:"error,omitempty"`
}

type Choice struct {
	Message Message `json:"message"`
}

type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// LLM structured response types
type Response struct {
	Type     string    `json:"type"`
	Commands []Command `json:"commands,omitempty"`
	// Config action fields
	Action string `json:"action,omitempty"`
	Key    string `json:"key,omitempty"`
	Value  string `json:"value,omitempty"`
}

type Command struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
	Certainty   int    `json:"certainty"`
}
