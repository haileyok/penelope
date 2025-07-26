package penelope

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/haileyok/penelope/letta"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	slogecho "github.com/samber/slog-echo"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Penelope struct {
	h           *http.Client
	x           *xrpc.Client
	letta       *letta.Client
	echo        *echo.Echo
	httpd       *http.Server
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
	apiKey      string
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
	LettaAgentName     string
	IgnoreDids         []string
	AdminOnly          bool
	ApiKey             string
	Addr               string
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
	db.AutoMigrate(
		&UserMemory{},
		&Block{},
	)

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
		Host:      args.LettaHost,
		ApiKey:    args.LettaApiKey,
		AgentName: args.LettaAgentName,
	})

	clock := syntax.NewTIDClock(0)

	e := echo.New()
	e.Pre(middleware.RemoveTrailingSlash())

	e.Use(middleware.Recover())
	e.Use(middleware.RemoveTrailingSlash())
	e.Use(echoprometheus.NewMiddleware(""))

	slogEchoCfg := slogecho.Config{
		DefaultLevel:     slog.LevelInfo,
		ServerErrorLevel: slog.LevelError,
		WithResponseBody: true,
		Filters: []slogecho.Filter{
			func(ctx echo.Context) bool {
				return ctx.Request().URL.Path != "/_health"
			},
		},
	}

	e.Use(slogecho.NewWithConfig(args.Logger, slogEchoCfg))

	httpd := &http.Server{
		Handler: e,
		Addr:    args.Addr,
	}

	return &Penelope{
		h:           h,
		x:           x,
		letta:       letta,
		echo:        e,
		httpd:       httpd,
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
		apiKey:      args.ApiKey,
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

	go func() {
		p.logger.Info("starting httpd")
		if err := p.httpd.ListenAndServe(); err != nil {
			p.logger.Error("httpd server failed", "error", err)
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

func (p *Penelope) handleAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(e echo.Context) error {
		auth := e.Request().Header.Get("authorization")
		pts := strings.Split(auth, " ")
		if len(pts) == 2 {
			if p.apiKey == pts[1] {
				return next(e)
			}
		}
		return e.JSON(http.StatusForbidden, makeErrorJson("unauthorized"))
	}
}

func (p *Penelope) addRoutes() {

	p.echo.GET("/_health", func(e echo.Context) error {
		return e.String(http.StatusOK, "healthy")
	})

	g := p.echo.Group("/tools")
	g.Use(p.handleAuthMiddleware)
	g.POST("/recent-posts", p.handleGetRecentPosts)
	g.POST("/create-top-level-post", p.handleCreateTopLevelPost)
	g.POST("/create-whitewind-post", p.handleCreateWhitewindPost)
}

type RequestError struct {
	Error string `json:"error"`
}

func makeErrorJson(error string) RequestError {
	return RequestError{
		Error: error,
	}
}
