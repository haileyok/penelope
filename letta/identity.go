package letta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/haileyok/penelope/letta/api"
)

func (c *Client) UpsertIdentity(ctx context.Context, input api.UpsertIdentityInput) error {
	input.AgentIDs = []string{c.agentName}

	b, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrJsonMarshal, err)
	}

	req, err := c.CreatePutRequest(ctx, "/v1/identities/", b)
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
