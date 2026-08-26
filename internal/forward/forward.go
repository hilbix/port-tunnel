package forward

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hilbix/port-tunnel/internal/config"
	"github.com/hilbix/port-tunnel/internal/protocol"
	"github.com/hilbix/port-tunnel/internal/tunnel"
)

type Forwarder struct {
	cfg    *config.Config
	tunnel *tunnel.Manager
	logger *slog.Logger

	listeners map[string]net.Listener

	wg sync.WaitGroup

	activeStreams atomic.Int64
}

func New(
	cfg *config.Config,
	manager *tunnel.Manager,
	logger *slog.Logger,
) *Forwarder {
	return &Forwarder{
		cfg:       cfg,
		tunnel:    manager,
		logger:    logger,
		listeners: make(map[string]net.Listener),
	}
}

func (f *Forwarder) Start(ctx context.Context) error {
	for _, listenerCfg := range f.cfg.Listeners {
		ln, err := net.Listen("tcp", listenerCfg.Listen)
		if err != nil {
			return fmt.Errorf(
				"listen %s (%s): %w",
				listenerCfg.Name,
				listenerCfg.Listen,
				err,
			)
		}

		f.listeners[listenerCfg.Name] = ln

		f.logger.Info(
			"listener started",
			"name", listenerCfg.Name,
			"address", listenerCfg.Listen,
		)

		f.wg.Add(1)

		go func(cfg config.ListenerConfig, ln net.Listener) {
			defer f.wg.Done()
			f.acceptLoop(ctx, cfg, ln)
		}(listenerCfg, ln)
	}

	f.tunnel.SetStreamHandler(f.handleIncomingStream)

	f.wg.Add(1)

	go func() {
		defer f.wg.Done()
		f.tunnel.AcceptStreams()
	}()

	return nil
}

func (f *Forwarder) Stop() {
	for name, ln := range f.listeners {
		if err := ln.Close(); err != nil {
			f.logger.Debug(
				"close listener",
				"name", name,
				"error", err,
			)
		}
	}

	f.wg.Wait()
}

func (f *Forwarder) acceptLoop(
	ctx context.Context,
	cfg config.ListenerConfig,
	ln net.Listener,
) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			if errors.Is(err, net.ErrClosed) {
				return
			}

			f.logger.Warn(
				"accept failed",
				"listener", cfg.Name,
				"error", err,
			)

			continue
		}

		f.wg.Add(1)

		go func() {
			defer f.wg.Done()

			f.handleLocalConnection(conn, cfg)
		}()
	}
}

func (f *Forwarder) handleLocalConnection(
	local net.Conn,
	cfg config.ListenerConfig,
) {
	defer local.Close()

	stream, err := f.tunnel.OpenStream()
	if err != nil {
		f.logger.Warn(
			"open tunnel stream failed",
			"listener", cfg.Name,
			"remote", local.RemoteAddr(),
			"error", err,
		)
		return
	}

	defer stream.Close()

	if err := protocol.WriteOpen(stream, cfg.Name); err != nil {
		f.logger.Warn(
			"write OPEN failed",
			"listener", cfg.Name,
			"error", err,
		)
		return
	}

	ok, remoteError, err := protocol.ReadOpenResponse(stream)
	if err != nil {
		f.logger.Warn(
			"read OPEN response failed",
			"listener", cfg.Name,
			"error", err,
		)
		return
	}

	if !ok {
		f.logger.Warn(
			"remote rejected connection",
			"listener", cfg.Name,
			"error", remoteError,
		)
		return
	}

	streams := f.activeStreams.Add(1)
	defer f.activeStreams.Add(-1)

	f.logger.Debug(
		"forwarding connection",
		"listener", cfg.Name,
		"remote", local.RemoteAddr(),
		"active_streams", streams,
	)

	err = proxy(local, stream)

	if err != nil {
		f.logger.Debug(
			"forwarding ended",
			"listener", cfg.Name,
			"error", err,
		)
	}
}

func (f *Forwarder) handleIncomingStream(stream net.Conn) {
	defer stream.Close()

	listenerID, err := protocol.ReadOpen(stream)
	if err != nil {
		f.logger.Warn(
			"invalid incoming stream",
			"error", err,
		)
		return
	}

	target, ok := f.cfg.Targets[listenerID]
	if !ok {
		f.logger.Warn(
			"unknown listener requested",
			"listener", listenerID,
		)

		_ = protocol.WriteOpenError(
			stream,
			"unknown listener: "+listenerID,
		)

		return
	}

	dialer := net.Dialer{
		Timeout: 10 * time.Second,
	}

	targetConn, err := dialer.DialContext(
		context.Background(),
		"tcp",
		target,
	)
	if err != nil {
		f.logger.Warn(
			"dial target failed",
			"listener", listenerID,
			"target", target,
			"error", err,
		)

		_ = protocol.WriteOpenError(
			stream,
			err.Error(),
		)

		return
	}

	defer targetConn.Close()

	if err := protocol.WriteOpenOK(stream); err != nil {
		return
	}

	streams := f.activeStreams.Add(1)
	defer f.activeStreams.Add(-1)

	f.logger.Debug(
		"accepted remote forwarding stream",
		"listener", listenerID,
		"target", target,
		"active_streams", streams,
	)

	if err := proxy(targetConn, stream); err != nil {
		f.logger.Debug(
			"remote forwarding ended",
			"listener", listenerID,
			"target", target,
			"error", err,
		)
	}
}

func proxy(a, b net.Conn) error {
	errCh := make(chan error, 2)

	go func() {
		_, err := io.Copy(b, a)
		halfCloseWrite(b)
		errCh <- err
	}()

	go func() {
		_, err := io.Copy(a, b)
		halfCloseWrite(a)
		errCh <- err
	}()

	err1 := <-errCh
	err2 := <-errCh

	if err1 != nil && !isNormalNetworkError(err1) {
		return err1
	}

	if err2 != nil && !isNormalNetworkError(err2) {
		return err2
	}

	return nil
}

func halfCloseWrite(conn net.Conn) {
	type closeWriter interface {
		CloseWrite() error
	}

	if cw, ok := conn.(closeWriter); ok {
		_ = cw.CloseWrite()
	}
}

func isNormalNetworkError(err error) bool {
	if err == nil {
		return true
	}

	if errors.Is(err, net.ErrClosed) {
		return true
	}

	return false
}
