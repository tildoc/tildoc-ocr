// Command tildoc-ocr recognises scanned PDFs for tildoc.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type handler func(ctx context.Context, c *client, j job, path, dir string) (results, error)

func worker(ctx context.Context, c *client, h handler, name string) {
	for ctx.Err() == nil {
		jobs, err := c.claim(ctx, 1)
		if err != nil {
			slog.Warn("claim", "plugin", name, "err", err)
		}
		if len(jobs) == 0 {
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Second):
			}
			continue
		}
		for _, j := range jobs {
			handle(ctx, c, h, j)
		}
	}
}

func handle(ctx context.Context, c *client, h handler, j job) {
	dir, err := os.MkdirTemp("", "tildoc-ocr-*")
	if err != nil {
		slog.Error("temp dir", "err", err)
		return
	}
	defer os.RemoveAll(dir)
	path := dir + "/in"
	r, err := func() (results, error) {
		if err := c.fetch(ctx, j, path); err != nil {
			return results{}, err
		}
		return h(ctx, c, j, path, dir)
	}()
	if ctx.Err() != nil {
		return // shutting down: the lease runs out and the job comes back
	}
	if err != nil {
		slog.Warn("job failed", "plugin", j.Plugin, "sha256", j.SHA256, "err", err)
		if err := c.fail(ctx, j, err.Error()); err != nil {
			slog.Error("report failure", "err", err)
		}
		return
	}
	if err := c.complete(ctx, j, r); err != nil {
		slog.Error("complete", "plugin", j.Plugin, "err", err)
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	base := os.Getenv("TILDOC_URL")
	ocrToken := os.Getenv("TILDOC_OCR_TOKEN")
	if base == "" || ocrToken == "" {
		slog.Error("TILDOC_URL and TILDOC_OCR_TOKEN are required")
		os.Exit(1)
	}
	n, err := strconv.Atoi(os.Getenv("TILDOC_OCR_CONCURRENCY"))
	if err != nil || n < 1 {
		n = 2
	}
	httpc := &http.Client{Timeout: 30 * time.Minute}
	ocr := &client{base: base, token: ocrToken, http: httpc}
	go ocr.publishSettings(ctx, []setting{
		{Key: "languages", Type: "multiselect", Options: availableLanguages(), Default: availableLanguages()},
		{Key: "preference", Type: "select", Options: []string{"existing", "tildoc"}, Default: "existing"},
		{Key: "redo_foreign", Type: "bool", Default: true},
	})
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() { defer wg.Done(); worker(ctx, ocr, runOCR, "tildoc-ocr") }()
	}
	slog.Info("tildoc-ocr started", "workers", n, "languages", availableLanguages())
	wg.Wait()
}
