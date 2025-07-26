package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/haileyok/penelope/penelope"
	_ "github.com/joho/godotenv/autoload"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.App{
		Name: "penelope",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "relay-host",
				EnvVars: []string{"PENELOPE_RELAY_HOST"},
				Value:   "wss://bsky.network",
			},
			&cli.StringFlag{
				Name:    "metrics-addr",
				EnvVars: []string{"PENELOPE_METRICS_ADDR"},
				Value:   ":8000",
			},
			&cli.BoolFlag{
				Name:    "debug",
				EnvVars: []string{"PENELOPE_DEBUG"},
			},
			&cli.StringFlag{
				Name:     "cursor-file",
				EnvVars:  []string{"PENELOPE_CURSOR_FILE"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "clickhouse-addr",
				EnvVars:  []string{"PENELOPE_CLICKHOUSE_ADDR"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "clickhouse-database",
				EnvVars:  []string{"PENELOPE_CLICKHOUSE_DATABASE"},
				Required: true,
			},
			&cli.StringFlag{
				Name:    "clickhouse-user",
				EnvVars: []string{"PENELOPE_CLICKHOUSE_USER"},
				Value:   "default",
			},
			&cli.StringFlag{
				Name:     "clickhouse-pass",
				EnvVars:  []string{"PENELOPE_CLICKHOUSE_PASS"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "bot-did",
				EnvVars:  []string{"PENELOPE_BOT_DID"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "bot-identifier",
				EnvVars:  []string{"PENELOPE_BOT_IDENTIFIER"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "bot-password",
				EnvVars:  []string{"PENELOPE_BOT_PASSWORD"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "bot-pds-host",
				EnvVars:  []string{"PENELOPE_BOT_PDS_HOST"},
				Required: true,
			},
			&cli.StringSliceFlag{
				Name:     "bot-admins",
				EnvVars:  []string{"PENELOPE_BOT_ADMINS"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "letta-host",
				EnvVars:  []string{"PENELOPE_LETTA_HOST"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "letta-api-key",
				EnvVars:  []string{"PENELOPE_LETTA_API_KEY"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "letta-agent-name",
				EnvVars:  []string{"PENELOPE_LETTA_AGENT_NAME"},
				Required: true,
			},
			&cli.StringSliceFlag{
				Name:     "ignore-dids",
				EnvVars:  []string{"PENELOPE_IGNORE_DIDS"},
				Required: true,
			},
			&cli.BoolFlag{
				Name:    "admin-only",
				EnvVars: []string{"PENELOPE_ADMIN_ONLY"},
			},
			&cli.StringFlag{
				Name:     "api-key",
				EnvVars:  []string{"PENELOPE_API_KEY"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "addr",
				EnvVars:  []string{"PENELOPE_ADDR"},
				Required: true,
			},
		},
		Commands: cli.Commands{
			&cli.Command{
				Name:   "run",
				Action: run,
			},
		},
		ErrWriter: os.Stderr,
	}

	app.Run(os.Args)
}

var run = func(cmd *cli.Context) error {
	ctx := cmd.Context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	level := slog.LevelInfo
	if cmd.Bool("debug") {
		level = slog.LevelDebug
	}

	l := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	p, err := penelope.New(ctx, &penelope.Args{
		Logger:             l,
		RelayHost:          cmd.String("relay-host"),
		MetricsAddr:        cmd.String("metrics-addr"),
		CursorFile:         cmd.String("cursor-file"),
		ClickhouseAddr:     cmd.String("clickhouse-addr"),
		ClickhouseDatabase: cmd.String("clickhouse-database"),
		ClickhouseUser:     cmd.String("clickhouse-user"),
		ClickhousePass:     cmd.String("clickhouse-pass"),
		BotDid:             cmd.String("bot-did"),
		BotIdentifier:      cmd.String("bot-identifier"),
		BotPassword:        cmd.String("bot-password"),
		BotPdsHost:         cmd.String("bot-pds-host"),
		BotAdmins:          cmd.StringSlice("bot-admins"),
		LettaHost:          cmd.String("letta-host"),
		LettaApiKey:        cmd.String("letta-api-key"),
		LettaAgentName:     cmd.String("letta-agent-name"),
		IgnoreDids:         cmd.StringSlice("ignore-dids"),
		AdminOnly:          cmd.Bool("admin-only"),
		ApiKey:             cmd.String("api-key"),
		Addr:               cmd.String("addr"),
	})
	if err != nil {
		panic(err)
	}

	go func() {
		exitSignals := make(chan os.Signal, 1)
		signal.Notify(exitSignals, syscall.SIGINT, syscall.SIGTERM)

		sig := <-exitSignals

		l.Info("received os exit signal", "signal", sig)
		cancel()
	}()

	if err := p.Run(ctx); err != nil {
		panic(err)
	}

	return nil
}
