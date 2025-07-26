package penelope

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/lex/util"
	"github.com/labstack/echo/v4"
)

type CreateTopLevelPostInput struct {
	Text string `json:"text"`
}

func (p *Penelope) handleCreateTopLevelPost(e echo.Context) error {
	ctx := e.Request().Context()

	var input CreateTopLevelPostInput
	if err := e.Bind(&input); err != nil {
		return e.JSON(500, makeErrorJson("failed to bind request"))
	}

	if err := p.createTopLevelPost(ctx, input.Text); err != nil {
		p.logger.Error("could not make post", "error", err)
		return e.JSON(500, makeErrorJson("failed to create post"))
	}

	return e.NoContent(200)
}

func (p *Penelope) createTopLevelPost(ctx context.Context, text string) error {
	parents := []*atproto.RepoStrongRef{nil}
	var root *atproto.RepoStrongRef

	postTexts := []string{}
	var currentText string
	words := strings.Split(text, " ")
	for i, w := range words {
		currentText += w + " "
		if len(words)-1 == i || len(currentText) >= 250 {
			postTexts = append(postTexts, currentText)
			currentText = ""
		}
	}

	var writes []*atproto.RepoApplyWrites_Input_Writes_Elem
	for _, pt := range postTexts {
		pt = strings.TrimSpace(pt)
		if pt == "" {
			continue
		}

		rkey := p.clock.Next().String()
		post := bsky.FeedPost{
			Text:      pt,
			CreatedAt: syntax.DatetimeNow().String(),
		}

		if parents[len(parents)-1] != nil {
			post.Reply = &bsky.FeedPost_ReplyRef{
				Parent: parents[len(parents)-1],
				Root:   root,
			}
		}

		writes = append(writes, &atproto.RepoApplyWrites_Input_Writes_Elem{
			RepoApplyWrites_Create: &atproto.RepoApplyWrites_Create{
				Collection: "app.bsky.feed.post",
				Rkey:       &rkey,
				Value:      &util.LexiconTypeDecoder{Val: &post},
			},
		})

		cborBytes := new(bytes.Buffer)
		err := post.MarshalCBOR(cborBytes)
		cidFromJson, err := cidbuilder.Sum(cborBytes.Bytes())
		if err != nil {
			p.logger.Error("error getting cid")
		}

		parents = append(parents, &atproto.RepoStrongRef{
			Uri: fmt.Sprintf("at://%s/app.bsky.feed.post/%s", p.botDid, rkey),
			Cid: cidFromJson.String(),
		})

		if root == nil {
			root = &atproto.RepoStrongRef{
				Uri: fmt.Sprintf("at://%s/app.bsky.feed.post/%s", p.botDid, rkey),
				Cid: cidFromJson.String(),
			}
		}
	}

	input := &atproto.RepoApplyWrites_Input{
		Repo:   p.botDid,
		Writes: writes,
	}

	_, err := atproto.RepoApplyWrites(ctx, p.x, input)
	if err != nil {
		p.logger.Error("error creating post", "error", err)
		return err
	}
	return nil

}
