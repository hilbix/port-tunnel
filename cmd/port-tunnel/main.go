package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hilbix/port-tunnel/internal/config"
	"github.com/hilbix/port-tunnel/internal/forward"
	"github.com/hilbix/port-tunnel/internal/tunnel"
)

func main() {
	var (
		configPath = flag.String(
			"config",
			"port-tunnel.yaml",
			"path to configuration file",
		)

		logLevel = flag.String(
			"log-level",
			"info",
			"log level: debug, info, warn, error",
		)
	)

	flag.Parse()

	level, err := parseLogLevel(*logLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	logger := slog.New(
		slog.NewTextHandler(
			os.Stderr,
			&slog.HandlerOptions{
				Level: level,
			},
		),
	)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error(
			"configuration error",
			"error", err,
		)
		os.Exit(1)
	}

	logger.Info(
		"starting port-tunnel",
		"node", cfg.Node.ID,
	)

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	manager := tunnel.NewManager(
		ctx,
		cfg,
		logger,
	)

	manager.Start()

	forwarder := forward.New(
		cfg,
		manager,
		logger,
	)

	if err := forwarder.Start(ctx); err != nil {
		logger.Error(
			"failed to start forwarding",
			"error", err,
		)

		manager.Stop()
		os.Exit(1)
	}

	tunnelListener, err := net.Listen(
		"tcp",
		cfg.Tunnel.Listen,
	)
	if err != nil {
		logger.Error(
			"failed to listen for tunnels",
			"address", cfg.Tunnel.Listen,
			"error", err,
		)

		forwarder.Stop()
		manager.Stop()
		os.Exit(1)
	}

	logger.Info(
		"tunnel listener started",
		"address", cfg.Tunnel.Listen,
	)

	go acceptTunnelConnections(
		ctx,
		tunnelListener,
		manager,
		logger,
	)

	<-ctx.Done()

	logger.Info("shutting down")

	_ = tunnelListener.Close()

	forwarder.Stop()
	manager.Stop()

	logger.Info("shutdown complete")

	// Give background logging a moment to flush in environments where
	// stderr is buffered by a supervisor.
	time.Sleep(50 * time.Millisecond)
}

func acceptTunnelConnections(
	ctx context.Context,
	ln net.Listener,
	manager *tunnel.Manager,
	logger *slog.Logger,
) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				logger.Warn(
					"temporary tunnel accept error",
					"error", err,
				)

				time.Sleep(100 * time.Millisecond)
				continue
			}

			logger.Debug(
				"tunnel listener closed",
				"error", err,
			)

			return
		}

		logger.Debug(
			"incoming tunnel connection",
			"remote", conn.RemoteAddr(),
		)

		manager.AcceptTransport(conn)
	}
}

func parseLogLevel(value string) (slog.Level, error) {
	switch value {
	case "debug":
		return slog.LevelDebug, nil

	case "info":
		return slog.LevelInfo, nil

	case "warn":
		return slog.LevelWarn, nil

	case "error":
		return slog.LevelError, nil

	default:
		return 0, fmt.Errorf(
			"unknown log level %q; expected debug, info, warn or error",
			value,
		)
	}
}
