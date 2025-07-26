package penelope

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/haileyok/penelope/letta"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Function struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
	Stream    bool      `json:"stream"`
	KeepAlive string    `json:"keep_alive"`
}

type ChatResponse struct {
	Model     string  `json:"model"`
	CreatedAt string  `json:"created_at"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
}

type FunctionCall struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments"`
}

type Penelope struct {
	h           *http.Client
	x           *xrpc.Client
	letta       *letta.Client
	xmu         sync.RWMutex
	conn        driver.Conn
	db          *gorm.DB
	cursorFile  string
	cursor      string
	logger      *slog.Logger
	relayHost   string
	metricsAddr string
	botDid      string
	botAdmins   []string
	processMu   sync.Mutex
	chatMu      sync.Mutex
	ignoreDids  []string
	clock       *syntax.TIDClock
	adminOnly   bool
}

type Args struct {
	ClickhouseAddr     string
	ClickhouseDatabase string
	ClickhouseUser     string
	ClickhousePass     string
	CursorFile         string
	Logger             *slog.Logger
	RelayHost          string
	MetricsAddr        string
	BotDid             string
	BotIdentifier      string
	BotPassword        string
	BotPdsHost         string
	BotAdmins          []string
	LettaHost          string
	LettaApiKey        string
	IgnoreDids         []string
	AdminOnly          bool
}

func New(ctx context.Context, args *Args) (*Penelope, error) {
	if args.Logger == nil {
		args.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	h := &http.Client{
		Timeout: 1200 * time.Second,
	}

	db, err := gorm.Open(sqlite.Open("penelope.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{args.ClickhouseAddr},
		Auth: clickhouse.Auth{
			Database: args.ClickhouseDatabase,
			Username: args.ClickhouseUser,
			Password: args.ClickhousePass,
		},
	})
	if err != nil {
		return nil, err
	}

	x := &xrpc.Client{
		Host: args.BotPdsHost,
	}

	args.Logger.Info("authenticating with pds...")

	resp, err := atproto.ServerCreateSession(ctx, x, &atproto.ServerCreateSession_Input{
		Identifier: args.BotIdentifier,
		Password:   args.BotPassword,
	})
	if err != nil {
		return nil, err
	}

	args.Logger.Info("authenticated with pds!")

	x.Auth = &xrpc.AuthInfo{
		AccessJwt:  resp.AccessJwt,
		RefreshJwt: resp.RefreshJwt,
		Handle:     resp.Handle,
		Did:        resp.Did,
	}

	letta, _ := letta.NewClient(&letta.ClientArgs{
		Host:   args.LettaHost,
		ApiKey: args.LettaApiKey,
	})

	clock := syntax.NewTIDClock(0)

	return &Penelope{
		h:           h,
		x:           x,
		letta:       letta,
		conn:        conn,
		db:          db,
		cursorFile:  args.CursorFile,
		logger:      args.Logger,
		relayHost:   args.RelayHost,
		metricsAddr: args.MetricsAddr,
		botDid:      args.BotDid,
		botAdmins:   args.BotAdmins,
		ignoreDids:  args.IgnoreDids,
		clock:       &clock,
		adminOnly:   args.AdminOnly,
	}, nil
}

func (p *Penelope) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	metricsServer := http.NewServeMux()
	metricsServer.Handle("/metrics", promhttp.Handler())

	go func() {
		p.logger.Info("Starting metrics server")
		if err := http.ListenAndServe(p.metricsAddr, metricsServer); err != nil {
			p.logger.Error("metrics server failed", "error", err)
		}
	}()

	go func(ctx context.Context, cancel context.CancelFunc) {
		p.logger.Info("starting relay", "relayHost", p.relayHost)
		if err := p.startConsumer(ctx, cancel); err != nil {
			panic(fmt.Errorf("failed to start consumer: %w", err))
		}
	}(ctx, cancel)

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			func() {
				p.xmu.Lock()
				defer p.xmu.Unlock()
				p.x.Auth.AccessJwt = p.x.Auth.RefreshJwt
				resp, err := atproto.ServerRefreshSession(ctx, p.x)
				if err != nil {
					p.logger.Error("error refreshing session", "error", err)
					return
				}
				p.x.Auth.AccessJwt = resp.AccessJwt
				p.x.Auth.RefreshJwt = resp.RefreshJwt
			}()
		}
	}()

	<-ctx.Done()

	return nil
}

func (p *Penelope) GetClient() *xrpc.Client {
	p.xmu.RLock()
	defer p.xmu.RUnlock()
	return p.x
}
