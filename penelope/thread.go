package penelope

import (
	"context"
	"fmt"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/haileyok/photocopy/models"
)

func (p *Penelope) LoadThread(ctx context.Context, reply *bsky.FeedPost_ReplyRef) (string, error) {
	if reply == nil {
		return "", nil
	}

	var posts []models.Post
	if err := p.conn.Select(ctx, &posts, "SELECT * FROM default.post WHERE (root_uri = ? OR uri = ?) AND created_at >= now() - interval 7 day", reply.Root.Uri, reply.Root.Uri); err != nil {
		return "", err
	}

	if len(posts) == 0 {
		return "", nil
	}

	postsMap := map[string]models.Post{}
	for _, p := range posts {
		postsMap[p.Uri] = p
	}

	var threadText string
	var its int
	nextUri := reply.Parent.Uri

	for its < 100 {
		p := postsMap[nextUri]
		threadText = fmt.Sprintf("<START_POST>By %s: %s<END_POST>\n", p.Did, p.Text) + threadText
		if p.ParentUri != "" {
			nextUri = p.ParentUri
			its++
		} else {
			break
		}
	}

	return threadText, nil

	// return p.SummarizeText(ctx, threadText)
}
