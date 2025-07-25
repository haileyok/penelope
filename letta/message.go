package letta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/haileyok/penelope/letta/api"
)

func (c *Client) SendMessage(ctx context.Context, messages []api.Message) (*api.MessageResult, error) {
	body := api.MessageInput{
		Messages:            messages,
		MaxSteps:            50,
		UseAssistantMessage: false,
		IncludeReturnMessageTypes: []api.MessageType{
			api.MessageToolCallMessage,
		},
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrJsonMarshal, err)
	}

	req, err := c.CreatePostRequest(ctx, "/v1/agents/:agent_id/messages", b)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRequest, err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrResponse, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("%w: status %d", ErrBadStatusCode, resp.StatusCode)
	}

	var result api.MessageResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrJsonUnmarshal, err)
	}

	return &result, nil
}

func (c *Client) ResetMessages(ctx context.Context) error {
	req, err := c.CreatePatchRequest(ctx, "/v1/agents/:agent_id/reset-messages")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRequest, err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrResponse, err)
	}
	defer resp.Body.Close()

	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrBadStatusCode, resp.StatusCode)
	}

	return nil
}
