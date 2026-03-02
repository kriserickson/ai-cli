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
	Type        string    `json:"type"`
	Explanation string    `json:"explanation,omitempty"`
	Commands    []Command `json:"commands,omitempty"`
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

type ChatTrace struct {
	Provider     string `json:"provider,omitempty"`
	Model        string `json:"model,omitempty"`
	Endpoint     string `json:"endpoint,omitempty"`
	RequestBody  string `json:"request_body,omitempty"`
	ResponseBody string `json:"response_body,omitempty"`
	RawContent   string `json:"raw_content,omitempty"`
	HTTPStatus   int    `json:"http_status,omitempty"`
}

type ChatResult struct {
	Response *Response
	Trace    ChatTrace
}
