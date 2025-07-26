package letta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/haileyok/penelope/letta/api"
)

func (c *Client) CreateBlock(ctx context.Context, input api.CreateBlockInput) (*api.CreateBlockResult, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrJsonMarshal, err)
	}

	req, err := c.CreatePostRequest(ctx, "/v1/blocks/", b)
	if err != nil {
		return nil, fmt.Errorf("%w, %w", ErrRequest, err)
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

	var result api.CreateBlockResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrJsonUnmarshal, err)
	}

	return &result, nil
}

func (c *Client) AttachBlock(ctx context.Context, blockId string) error {
	req, err := c.CreatePatchRequest(ctx, "/v1/agents/:agent_id/core-memory/blocks/attach/"+blockId)
	if err != nil {
		return fmt.Errorf("%w, %w", ErrRequest, err)
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

func (c *Client) DetachBlock(ctx context.Context, blockId string) error {
	req, err := c.CreatePatchRequest(ctx, "/v1/agents/:agent_id/core-memory/blocks/detach/"+blockId)
	if err != nil {
		return fmt.Errorf("%w, %w", ErrRequest, err)
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
