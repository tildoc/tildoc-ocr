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
	"time"
)

// job is one piece of work from tildoc.
type job struct {
	ID          string         `json:"id"`
	Plugin      string         `json:"plugin"`
	SHA256      string         `json:"sha256"`
	MIME        string         `json:"mime"`
	ContentPath string         `json:"content_path"`
	TextPath    string         `json:"text_path"`
	Force       bool           `json:"force"`
	VariantOf   string         `json:"variant_of"` // this content is a variant of that file
	Settings    map[string]any `json:"settings"`
}

// results are what a job reports.
type results struct {
	Status string `json:"status"` // done | partial | unsupported
}

// client talks to tildoc as one plugin.
type client struct {
	base, token string
	http        *http.Client
}

func (c *client) do(ctx context.Context, method, path string, body io.Reader, ctype string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
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

func (c *client) postJSON(ctx context.Context, path string, in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, path, bytes.NewReader(b), "application/json", out)
}

func (c *client) claim(ctx context.Context, n int) ([]job, error) {
	var jobs []job
	err := c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/plugin/jobs/claim?limit=%d", n), nil, "", &jobs)
	return jobs, err
}

// fetch stores a job's file at path.
func (c *client) fetch(ctx context.Context, j job, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+j.ContentPath, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %d", resp.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return f.Close()
}

// text returns the extracted text of a job's file; "" if there is none yet.
func (c *client) text(ctx context.Context, j job) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+j.TextPath, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("text: %d", resp.StatusCode)
	}
	var out struct {
		Text string `json:"text"`
	}
	return out.Text, json.NewDecoder(resp.Body).Decode(&out)
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
	err = c.do(ctx, http.MethodPut, "/api/v1/plugin/jobs/"+j.ID+"/variants/"+url.PathEscape(v.Kind)+"?"+q.Encode(), f, "application/octet-stream", &out)
	return out.SHA256, err
}

func (c *client) complete(ctx context.Context, j job, r results) error {
	return c.postJSON(ctx, "/api/v1/plugin/jobs/"+j.ID+"/complete", r, nil)
}

func (c *client) fail(ctx context.Context, j job, msg string) error {
	return c.postJSON(ctx, "/api/v1/plugin/jobs/"+j.ID+"/fail", map[string]any{"error": msg, "retry": false}, nil)
}

// setting is one option shown on tildoc's Administration page.
type setting struct {
	Key     string         `json:"key"`
	Type    string         `json:"type"` // select | multiselect | bool
	Options []string       `json:"options,omitempty"`
	Default any            `json:"default"`
	Extra   map[string]any `json:"-"`
}

// publishSettings tells tildoc which settings exist, retrying until it answers.
func (c *client) publishSettings(ctx context.Context, s []setting) {
	for ctx.Err() == nil {
		b, _ := json.Marshal(map[string]any{"settings": s})
		if err := c.do(ctx, http.MethodPut, "/api/v1/plugin/settings", bytes.NewReader(b), "application/json", nil); err == nil {
			return
		}
		select {
		case <-ctx.Done():
		case <-time.After(3 * time.Second):
		}
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
