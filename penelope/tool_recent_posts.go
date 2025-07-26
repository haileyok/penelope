package penelope

import (
	"context"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/labstack/echo/v4"
)

type GetRecentPostsInput struct {
	Did string `json:"did"`
}

type GetRecentPostsResponse struct {
	Posts string `json:"posts"`
}

func (p *Penelope) handleGetRecentPosts(e echo.Context) error {
	ctx := e.Request().Context()

	var input GetRecentPostsInput
	if err := e.Bind(&input); err != nil {
		return e.JSON(500, makeErrorJson("failed to bind request"))
	}

	posts, err := p.getUserRecentPosts(ctx, input.Did)
	if err != nil {
		return e.JSON(500, makeErrorJson("failed to get recent posts"))
	}

	var postsText string
	postsText += "<BEGIN POSTS>\n"

	for _, p := range posts {
		if p.Post.Author.Did != input.Did {
			continue
		}
		fp := p.Post.Record.Val.(*bsky.FeedPost)
		postsText += "<BEGIN POST>" + fp.Text + "<END POST>\n"
	}

	postsText += "<END POSTS>"

	return e.JSON(200, GetRecentPostsResponse{
		Posts: postsText,
	})
}

func (p *Penelope) getUserRecentPosts(ctx context.Context, did string) ([]*bsky.FeedDefs_FeedViewPost, error) {
	resp, err := bsky.FeedGetAuthorFeed(ctx, p.GetClient(), did, "", "posts_and_author_threads", true, 50)
	if err != nil {
		return nil, err
	}
	return resp.Feed, nil
}
