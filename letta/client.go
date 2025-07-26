package letta

import (
	"bytes"
	"context"
	"net/http"

	"github.com/bluesky-social/indigo/pkg/robusthttp"
)

type Client struct {
	client *http.Client
	host   string
	apiKey string
}

type ClientArgs struct {
	Host   string
	ApiKey string
}

func NewClient(args *ClientArgs) (*Client, error) {
	return &Client{
		client: robusthttp.NewClient(),
		host:   args.Host,
		apiKey: args.ApiKey,
	}, nil
}

func (c *Client) CreatePostRequest(ctx context.Context, endpoint string, bodyBytes []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.host+endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+c.apiKey)

	return req, nil
}

func (c *Client) CreateGetRequest(ctx context.Context, endpoint string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.host+endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "application/json")
	return req, nil
}
