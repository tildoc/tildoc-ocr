package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// job is one piece of work from tildoc.
type job struct {
	ID         string         `json:"id"`
	Plugin     string         `json:"plugin"`
	SHA256     string         `json:"sha256"`
	MIME       string         `json:"mime"`
	Force      bool           `json:"force"`
	VariantOf  string         `json:"variant_of"` // this content is a variant of that file
	Settings   map[string]any `json:"settings"`
	ReplyToken string         `json:"reply_token"`
	Text       string         `json:"-"` // the extracted text, if the plugin asked for it
}

// results are what a job reports.
type results struct {
	Status string `json:"status"` // done | partial | unsupported
}

// client delivers results to tildoc, with the token that came with each job.
type client struct {
	base string
	http *http.Client
}

func (c *client) do(ctx context.Context, token, method, path string, body io.Reader, ctype string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%s %s: %d %s", method, path, resp.StatusCode, msg)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *client) postJSON(ctx context.Context, token, path string, in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return c.do(ctx, token, http.MethodPost, path, bytes.NewReader(b), "application/json", out)
}

// variant describes a derived file.
type variant struct {
	Kind, Input, Engine, Description string
	ProvidesText                     bool
	Meta                             map[string]any
}

// putVariant uploads a derived file and returns its hash.
func (c *client) putVariant(ctx context.Context, j job, v variant, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	meta, err := json.Marshal(v.Meta)
	if err != nil {
		return "", err
	}
	q := url.Values{"input": {v.Input}, "engine": {v.Engine}, "description": {v.Description}, "meta": {string(meta)}}
	if v.ProvidesText {
		q.Set("provides_text", "1")
	}
	var out struct {
		SHA256 string `json:"sha256"`
	}
	err = c.do(ctx, j.ReplyToken, http.MethodPut, "/api/v1/plugin/jobs/"+j.ID+"/variants/"+url.PathEscape(v.Kind)+"?"+q.Encode(), f, "application/octet-stream", &out)
	return out.SHA256, err
}

func (c *client) complete(ctx context.Context, j job, r results) error {
	return c.postJSON(ctx, j.ReplyToken, "/api/v1/plugin/jobs/"+j.ID+"/complete", r, nil)
}

func (c *client) fail(ctx context.Context, j job, msg string) error {
	return c.postJSON(ctx, j.ReplyToken, "/api/v1/plugin/jobs/"+j.ID+"/fail", map[string]any{"error": msg, "retry": false}, nil)
}

// setting is one option shown on tildoc's Administration page.
type setting struct {
	Key     string         `json:"key"`
	Type    string         `json:"type"` // select | multiselect | bool
	Options []string       `json:"options,omitempty"`
	Default any            `json:"default"`
	Extra   map[string]any `json:"-"`
}

func ocrSettings() []setting {
	return []setting{
		{Key: "languages", Type: "multiselect", Options: availableLanguages(), Default: availableLanguages()},
		{Key: "preference", Type: "select", Options: []string{"existing", "tildoc"}, Default: "existing"},
		{Key: "redo_foreign", Type: "bool", Default: true},
	}
}

func availableLanguages() []string {
	var out []string
	for _, l := range strings.Split(os.Getenv("OCR_LANGS"), ",") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
