package penelope

import (
	"context"
	"errors"
	"time"

	"bytes"
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/lex/util"
	"github.com/haileyok/penelope/letta/api"
	gocid "github.com/ipfs/go-cid"
	"gorm.io/gorm"
	"mvdan.cc/xurls/v2"
)

var cidbuilder = gocid.V1Builder{Codec: 0x71, MhType: 0x12, MhLength: 0}

func (p *Penelope) SendMessage(ctx context.Context, rec *bsky.FeedPost, did, uri, cid, c string) {
	p.chatMu.Lock()

	var block Block
	defer func(ctx context.Context) {
		if err := p.letta.DetachBlock(ctx, block.Id); err != nil {
			p.logger.Error("could not detatch block from agent", "error", err)
		}
		if err := p.letta.ResetMessages(ctx); err != nil {
			p.logger.Error("could not reset message", "error", err)
		}
		p.chatMu.Unlock()
	}(ctx)

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	profile, err := bsky.ActorGetProfile(ctx, p.GetClient(), did)
	if err != nil {
		p.logger.Error("failed to get user profile", "error", err)
		return
	}

	identityProperties := []api.IdentityProperty{
		{Key: "did", Value: did, Type: "string"}, {Key: "handle", Value: profile.Handle},
	}

	var displayName string
	if profile.DisplayName != nil {
		identityProperties = append(identityProperties, api.IdentityProperty{
			Key:   "display-name",
			Value: profile.DisplayName,
			Type:  "string",
		})
		displayName = *profile.DisplayName
	}

	var description string
	if profile.Description != nil {
		identityProperties = append(identityProperties, api.IdentityProperty{
			Key:   "description",
			Value: profile.Description,
			Type:  "string",
		})
		description = *profile.Description
	}

	if err := p.db.Raw("SELECT * FROM blocks WHERE did = ?", did).Scan(&block).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			p.logger.Error("error getting block from db", "error", err)
			return
		}
	}

	if block.Id == "" {
		var currentMemories string
		memories, err := p.getUserMemory(did)
		if err == nil && memories != "" {
			p.logger.Info("found existing memories for user, migrating", "did", did)
			summary, err := p.SummarizeText(ctx, memories)
			if err != nil {
				p.logger.Error("could not summarize memories", "error", err)
			}
			p.logger.Info("summarized memories", "summary", summary)
			currentMemories = summary
		}

		newBlock, err := p.letta.CreateBlock(ctx, api.CreateBlockInput{
			Value: fmt.Sprintf(UserBlockValue, profile.Handle, did, displayName, description, currentMemories),
			Label: "user-" + did,
			Limit: 15000,
		})
		if err != nil {
			p.logger.Error("could not create block", "error", err)
			return
		}

		if newBlock.ID == nil {
			p.logger.Error("unexpected nil id for new block", "block", block)
			return
		}

		block = Block{
			Did: did,
			Id:  *newBlock.ID,
		}
		if err := p.db.Create(&block).Error; err != nil {
			p.logger.Error("could not add new block to db", "error", err)
			return
		}

		p.logger.Info("created memory block for user", "did", did, "block-id", block.Id)
	} else {
		p.logger.Info("found memory block id for user", "did", did, "block-id", block.Id)
	}

	threadSummary, err := p.LoadThread(ctx, rec.Reply)
	if err != nil {
		p.logger.Error("could not load thread", "error", err)
		return
	}

	if err := p.letta.AttachBlock(ctx, block.Id); err != nil {
		p.logger.Error("could not attach block to agent", "error", err)
		return
	}

	var content string
	if threadSummary != "" {
		content += "<thread_summary>" + threadSummary + "</thread_summary>\n\n"
	}
	content += rec.Text

	resp, err := p.letta.SendMessage(ctx, []api.Message{
		{
			Role:     "user",
			Content:  content,
			SenderID: &did,
		},
	})
	if err != nil {
		p.logger.Error("error sending message", "error", err)
		return
	}

	parents := []*atproto.RepoStrongRef{{
		Uri: uri,
		Cid: cid,
	}}

	var root *atproto.RepoStrongRef
	if rec.Reply != nil {
		root = rec.Reply.Root
	} else {
		root = &atproto.RepoStrongRef{
			Uri: uri,
			Cid: cid,
		}
	}

	if len(resp.Messages) == 0 {
		p.logger.Error("message response contained more than one message", "messages-length", len(resp.Messages))
		return
	}

	var response string
	for _, m := range resp.Messages {
		if m.MessageType != string(api.MessageToolCallMessage) {
			continue
		}
		arguments, err := api.ParseToolCallArguments(m.ToolCall.Arguments)
		if err != nil {
			p.logger.Error("error parsing arguments", "error", err)
		}
		response = arguments.Message
	}

	postTexts := []string{}
	var currentText string
	words := strings.Split(response, " ")
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
			Reply: &bsky.FeedPost_ReplyRef{
				Parent: parents[len(parents)-1],
				Root:   root,
			},
		}

		strict := xurls.Strict()
		urls := strict.FindAllString(pt, -1)
		if len(urls) != 0 {
			post.Embed = &bsky.FeedPost_Embed{
				EmbedExternal: &bsky.EmbedExternal{
					External: &bsky.EmbedExternal_External{
						Uri: urls[0],
					},
				},
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
		err = post.MarshalCBOR(cborBytes)
		cidFromJson, err := cidbuilder.Sum(cborBytes.Bytes())
		if err != nil {
			p.logger.Error("error getting cid")
		}

		parents = append(parents, &atproto.RepoStrongRef{
			Uri: fmt.Sprintf("at://%s/app.bsky.feed.post/%s", p.botDid, rkey),
			Cid: cidFromJson.String(),
		})
	}

	input := &atproto.RepoApplyWrites_Input{
		Repo:   p.botDid,
		Writes: writes,
	}

	_, err = atproto.RepoApplyWrites(ctx, p.x, input)
	if err != nil {
		p.logger.Error("error creating post", "error", err)
		return
	}

	p.logger.Info("replying to post with message", "msg", response)
}

const (
	UserBlockValue = `This is my section of core memory devoted to information about the user.
	I currently know the following about them:
	Bluesky Handle: @%s
	Atproto DID: %s
	Display Name: %s
	Profile Description: %s
	Where are they from? What do they do? Who are they? What do they post about?
	I should update this memory over time as I interact with the human and learn more about them.

	%s
	`
)

func (p *Penelope) SummarizeText(ctx context.Context, text string) (string, error) {
	defer func() {
		p.letta.ResetMessages(ctx)
	}()

	resp, err := p.letta.SendMessage(ctx, []api.Message{
		{
			Role:    "user",
			Content: "Please take the following text and form a 1-3 paragraph summary of it. You shouldn't feel like you need to make it too short, but stay under 3 paragraphs if possible.\n\n" + text,
		},
	})
	if err != nil {
		p.logger.Error("error sending message", "error", err)
		return "", nil
	}

	if len(resp.Messages) == 0 {
		return "", fmt.Errorf("error summarizing text. response was empty")
	}

	var response string
	for _, m := range resp.Messages {
		if m.MessageType != string(api.MessageToolCallMessage) {
			continue
		}
		arguments, err := api.ParseToolCallArguments(m.ToolCall.Arguments)
		if err != nil {
			return "", fmt.Errorf("error parsing arguments: %w", err)
		}
		response = arguments.Message
	}

	return response, nil
}
