package api

type CreateBlockInput struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Limit int    `json:"limit"`
}

type CreateBlockResult struct {
	Value string  `json:"value"`
	Label string  `json:"label"`
	Limit int     `json:"limit"`
	ID    *string `json:"id,omitempty"`
}

type AttachBlockInput struct {
	AgentID string `json:"agent_id"`
	BlockID string `json:"block_id"`
}

type DetatchBlockInput struct {
	AgentID string `json:"agent_id"`
	BlockID string `json:"block_id"`
}
