// Command tildoc-ocr recognises scanned PDFs for tildoc.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type handler func(ctx context.Context, c *client, j job, path, dir string) (results, error)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	base := os.Getenv("TILDOC_URL")
	if base == "" {
		slog.Error("TILDOC_URL is required")
		os.Exit(1)
	}
	listen := os.Getenv("PLUGIN_LISTEN")
	if listen == "" {
		listen = ":8000"
	}
	n, err := strconv.Atoi(os.Getenv("PLUGIN_CONCURRENCY"))
	if err != nil || n < 1 {
		n = 2
	}
	s := &server{ctx: ctx, client: &client{base: base, http: &http.Client{Timeout: 30 * time.Minute}},
		slots: make(chan struct{}, n), run: runOCR, settings: ocrSettings()}
	srv := &http.Server{Addr: listen, Handler: s.routes(), ReadHeaderTimeout: 30 * time.Second}
	go func() {
		<-ctx.Done()
		sd, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sd)
	}()
	slog.Info("tildoc-ocr started", "listen", listen, "workers", n)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
