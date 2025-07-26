package penelope

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type CreateWhitewindPostInput struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type CreateWhitewindPostResponse struct {
	Url string `json:"url"`
}

func (p *Penelope) handleCreateWhitewindPost(e echo.Context) error {
	ctx := e.Request().Context()

	var input CreateWhitewindPostInput
	if err := e.Bind(&input); err != nil {
		return e.JSON(200, makeErrorJson("failed to bind request"))
	}

	resp, err := p.createWhitewindPost(ctx, input.Title, input.Text)
	if err != nil {
		p.logger.Error("failed to create whitewind post", "error", err)
		return e.JSON(500, makeErrorJson("failed to create whitewind post"))
	}

	return e.JSON(200, CreateWhitewindPostResponse{
		Url: resp,
	})
}

type CreateRecordRequest struct {
	Repo       string `json:"repo"`
	Collection string `json:"collection"`
	Rkey       string `json:"rkey"`
	Record     any    `json:"record"`
}

type WhitewindRecord struct {
	LexiconTypeID string `json:"$type,const=com.whtwnd.blog.entry" cborgen:"$type,const=com.whtwnd.blog.entry"`
	Content       string `json:"content"`
	CreatedAt     string `json:"createdAt"`
	Theme         string `json:"theme"`
	Title         string `json:"title"`
	Visibility    string `json:"visibility"`
	Subtitle      string `json:"subtitle"`
}

func (p *Penelope) createWhitewindPost(ctx context.Context, title, content string) (string, error) {
	content = strings.ReplaceAll(content, "<BEGIN_WHITEWIND_CONTENT>", "")
	content = strings.ReplaceAll(content, "<END_WHITEWIND_CONTENT>", "")
	content = strings.TrimSpace(content)

	rkey := p.clock.Next().String()
	rec := WhitewindRecord{
		LexiconTypeID: "com.whtwnd.blog.entry",
		CreatedAt:     time.Now().Format(time.RFC3339Nano),
		Content:       content,
		Title:         title,
		Visibility:    "url",
		Theme:         "github-light",
	}

	input := CreateRecordRequest{
		Collection: "com.whtwnd.blog.entry",
		Record:     rec,
		Repo:       p.botDid,
		Rkey:       rkey,
	}

	b, _ := json.Marshal(input)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://cocoon.hailey.at/xrpc/com.atproto.repo.createRecord", bytes.NewReader(b))
	if err != nil {
		return "", err
	}

	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+p.x.Auth.AccessJwt)

	_, err = p.h.Do(req)
	if err != nil {
		return "", err
	}

	return "https://whtwnd.com/" + p.botDid + "/" + rkey, nil
}
