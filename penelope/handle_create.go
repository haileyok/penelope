package penelope

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/araddon/dateparse"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func (p *Penelope) handleCreate(ctx context.Context, recb []byte, indexedAt, rev, did, collection, rkey, cid, seq string) error {
	iat, err := dateparse.ParseAny(indexedAt)
	if err != nil {
		return err
	}

	switch collection {
	case "app.bsky.feed.post":
		return p.handleCreatePost(ctx, rev, recb, uriFromParts(did, collection, rkey), did, collection, rkey, cid, iat)
	default:
		return nil
	}
}

func (p *Penelope) handleCreatePost(ctx context.Context, rev string, recb []byte, uri, did, collection, rkey, cid string, indexedAt time.Time) error {
	if did == p.botDid {
		return nil
	}

	var rec bsky.FeedPost
	if err := rec.UnmarshalCBOR(bytes.NewReader(recb)); err != nil {
		return err
	}

	if p.adminOnly {
		isAdmin := slices.Contains(p.botAdmins, did)
		if !isAdmin {
			return nil
		}
	}

	var mentionsDid bool
	for _, f := range rec.Facets {
		for _, ff := range f.Features {
			if ff.RichtextFacet_Mention == nil {
				continue
			}
			if ff.RichtextFacet_Mention.Did == p.botDid {
				mentionsDid = true
			}
		}
	}

	if !mentionsDid && rec.Reply != nil && rec.Reply.Root != nil && rec.Reply.Parent != nil {
		rootUri, err := syntax.ParseATURI(rec.Reply.Root.Uri)
		if err != nil {
			return err
		}

		parentUri, err := syntax.ParseATURI(rec.Reply.Parent.Uri)
		if err != nil {
			return err
		}

		if rootUri.Authority().String() != p.botDid && parentUri.Authority().String() != p.botDid {
			return nil
		}

		if parentUri.Authority().String() == did {
			return fmt.Errorf("skipping this post because it is a consecutive thread reply")
		}
	} else if !mentionsDid {
		return nil
	}

	if slices.Contains(p.ignoreDids, did) {
		return fmt.Errorf("post from an ignored user")
	}

	p.logger.Info("got a post to reply to", "uri", uri)

	go p.SendMessage(ctx, &rec, did, uri, cid, rec.Text)

	return nil
}

func parseTimeFromRecord(rec any, rkey string) (*time.Time, error) {
	var rkeyTime time.Time
	if rkey != "self" {
		rt, err := syntax.ParseTID(rkey)
		if err == nil {
			rkeyTime = rt.Time()
		}
	}
	switch rec := rec.(type) {
	case *bsky.FeedPost:
		t, err := dateparse.ParseAny(rec.CreatedAt)
		if err != nil {
			return nil, err
		}

		if inRange(t) {
			return &t, nil
		}

		if rkeyTime.IsZero() || !inRange(rkeyTime) {
			return timePtr(time.Now()), nil
		}

		return &rkeyTime, nil
	case *bsky.FeedRepost:
		t, err := dateparse.ParseAny(rec.CreatedAt)
		if err != nil {
			return nil, err
		}

		if inRange(t) {
			return timePtr(t), nil
		}

		if rkeyTime.IsZero() {
			return nil, fmt.Errorf("failed to get a useful timestamp from record")
		}

		return &rkeyTime, nil
	case *bsky.FeedLike:
		t, err := dateparse.ParseAny(rec.CreatedAt)
		if err != nil {
			return nil, err
		}

		if inRange(t) {
			return timePtr(t), nil
		}

		if rkeyTime.IsZero() {
			return nil, fmt.Errorf("failed to get a useful timestamp from record")
		}

		return &rkeyTime, nil
	case *bsky.ActorProfile:
		// We can't really trust the createdat in the profile record anyway, and its very possible its missing. just use iat for this one
		return timePtr(time.Now()), nil
	case *bsky.FeedGenerator:
		if !rkeyTime.IsZero() && inRange(rkeyTime) {
			return &rkeyTime, nil
		}
		return timePtr(time.Now()), nil
	default:
		if !rkeyTime.IsZero() && inRange(rkeyTime) {
			return &rkeyTime, nil
		}
		return timePtr(time.Now()), nil
	}
}

func inRange(t time.Time) bool {
	now := time.Now()
	if t.Before(now) {
		return now.Sub(t) <= time.Hour*24*365*5
	}
	return t.Sub(now) <= time.Hour*24*200
}

func timePtr(t time.Time) *time.Time {
	return &t
}
