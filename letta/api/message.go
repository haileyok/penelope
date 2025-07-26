package api

import "time"

type Message struct {
	Role        string  `json:"role"`
	Content     string  `json:"content"` // TODO: this can also be an array of `MessageCreateContent`, should refactor later
	Name        *string `json:"name,omitempty"`
	Otid        *string `json:"otid,omitempty"`
	SenderID    *string `json:"sender_id"` // NOTE: can this be a DID?
	BatchItemID *string `json:"batch_item_id"`
	GroupID     *string `json:"group_id"`
}

type MessageInput struct {
	Messages                  []Message     `json:"messages"`
	MaxSteps                  int           `json:"max_steps"`
	UseAssistantMessage       bool          `json:"use_assistant_message"`
	AssistantMessageToolName  *string       `json:"assistant_message_tool_name,omitempty"`
	AssistantMessageToolKwarg *string       `json:"assistant_message_tool_kwarg,omitempty"`
	IncludeReturnMessageTypes []MessageType `json:"include_return_message_types,omitempty"`
}

type MessageType string

var (
	MessageSystemMessage          = MessageType("system_message")
	MessageUserMessage            = MessageType("user_message")
	MessageAssistantMessage       = MessageType("assistant_message")
	MessageReasoningMessage       = MessageType("reasoning_message")
	MessageHiddenReasoningMessage = MessageType("hidden_reasoning_message")
	MessageToolCallMessage        = MessageType("tool_call_message")
	MessageToolReturnMessage      = MessageType("tool_return_message")
)

type MessageResult struct {
	Messages []struct {
		ID          string    `json:"id"`
		Date        time.Time `json:"date"`
		Name        string    `json:"name"`
		MessageType string    `json:"message_type"`
		Otid        string    `json:"otid"`
		SenderID    string    `json:"sender_id"`
		StepID      string    `json:"step_id"`
		IsErr       bool      `json:"is_err"`
		Content     string    `json:"content"`
	} `json:"messages"`
	StopReason struct {
		StopReason  string `json:"stop_reason"`
		MessageType string `json:"message_type"`
	} `json:"stop_reason"`
	Usage struct {
		MessageType      string `json:"message_type"`
		CompletionTokens int    `json:"completion_tokens"`
		PromptTokens     int    `json:"prompt_tokens"`
		TotalTokens      int    `json:"total_tokens"`
		StepCount        int    `json:"step_count"`
		StepsMessages    [][]struct {
			Role            string    `json:"role"`
			CreatedByID     string    `json:"created_by_id"`
			LastUpdatedByID string    `json:"last_updated_by_id"`
			CreatedAt       time.Time `json:"created_at"`
			UpdatedAt       time.Time `json:"updated_at"`
			ID              string    `json:"id"`
			AgentID         string    `json:"agent_id"`
			Model           string    `json:"model"`
			Content         []struct {
				Type string `json:"type"`
			} `json:"content"`
			Name      string `json:"name"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Arguments string `json:"arguments"`
					Name      string `json:"name"`
				} `json:"function"`
				Type string `json:"type"`
			} `json:"tool_calls"`
			ToolCallID  string `json:"tool_call_id"`
			StepID      string `json:"step_id"`
			Otid        string `json:"otid"`
			ToolReturns []struct {
				Status string   `json:"status"`
				Stdout []string `json:"stdout"`
				Stderr []string `json:"stderr"`
			} `json:"tool_returns"`
			GroupID     string `json:"group_id"`
			SenderID    string `json:"sender_id"`
			BatchItemID string `json:"batch_item_id"`
			IsErr       bool   `json:"is_err"`
		} `json:"steps_messages"`
		RunIds []string `json:"run_ids"`
	} `json:"usage"`
}
