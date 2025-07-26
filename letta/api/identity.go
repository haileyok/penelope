package api

type UpsertIdentityInput struct {
	IdentifierKey string             `json:"identifier_key"`
	Name          string             `json:"name"`
	IdentityType  string             `json:"identity_type"`
	Properties    []IdentityProperty `json:"properties"`
	AgentIDs      []string           `json:"agent_ids"`
}

type IdentityProperty struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
	Type  string `json:"string"`
}
