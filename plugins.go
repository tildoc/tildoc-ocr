package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func textFact(kind string, pages int) []fact {
	f := []fact{{Label: "fact.kind", Value: "kind." + kind}}
	if pages > 0 {
		f = append(f, fact{Label: "fact.pages", Value: strconv.Itoa(pages)})
	}
	return f
}

// runText is the tildoc-text plugin.
func runText(ctx context.Context, j job, path string) (results, error) {
	head, err := readHead(path, 8192)
	if err != nil {
		return results{}, err
	}
	if j.MIME != "application/pdf" {
		if !looksLikeText(head) {
			return results{Status: "unsupported"}, nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return results{}, err
		}
		if len(b) > maxTextBytes {
			b = b[:maxTextBytes]
		}
		text := string(b)
		return results{Status: "done", Facts: textFact("text", 0), Text: &text}, nil
	}
	text, err := pdfText(ctx, path)
	if err != nil {
		return results{}, err
	}
	pages, empty := pageStats(text)
	kind := "pdf-digital"
	switch {
	case j.OwnVariant:
		kind = "pdf-own"
	case empty == pages:
		kind = "pdf-scanned"
	case empty > 0:
		kind = "pdf-mixed"
	case foreignOCR(ctx, path, pages):
		kind = "pdf-ocred"
	}
	r := results{Status: "done", Facts: textFact(kind, pages)}
	if strings.TrimSpace(text) != "" {
		r.Text = &text
	}
	return r, nil
}

func settingString(j job, key, def string) string {
	if v, ok := j.Settings[key].(string); ok && v != "" {
		return v
	}
	return def
}

func settingBool(j job, key string, def bool) bool {
	if v, ok := j.Settings[key].(bool); ok {
		return v
	}
	return def
}

// languages joins the chosen languages for Tesseract, never outside what the image has.
func languages(j job, have []string) string {
	var out []string
	if list, ok := j.Settings["languages"].([]any); ok {
		for _, x := range list {
			for _, h := range have {
				if x == h {
					out = append(out, h)
				}
			}
		}
	}
	if len(out) == 0 {
		out = have
	}
	return strings.Join(out, "+")
}

func fileSHA(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

// runOCR is the tildoc-ocr plugin.
func runOCR(ctx context.Context, c *client, j job, path, dir string) (results, error) {
	if j.OwnVariant { // our own result used as content again: nothing to recognise
		return results{Status: "unsupported"}, nil
	}
	text, err := pdfText(ctx, path)
	if err != nil {
		return results{}, err
	}
	pages, empty := pageStats(text)
	langs := languages(j, availableLanguages())
	if empty > 0 {
		return recognise(ctx, c, j, path, dir, variant{Kind: "searchable_pdf", Input: j.SHA256, Description: "description.skip_text",
			Meta: map[string]any{"langs": langs}}, []string{"--skip-text"}, langs, true)
	}
	if !foreignOCR(ctx, path, pages) || !settingBool(j, "redo_foreign", true) {
		return results{Status: "unsupported"}, nil
	}
	// someone else's OCR: an image-only variant, then our own recognition of it
	img := filepath.Join(dir, "image-only.pdf")
	if _, err := run(ctx, "gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=pdfwrite", "-dFILTERTEXT",
		"-dPassThroughJPEGImages=true", "-dPassThroughJPXImages=true", "-dDownsampleColorImages=false",
		"-dDownsampleGrayImages=false", "-dDownsampleMonoImages=false", "-sOutputFile="+img, path); err != nil {
		return results{}, fmt.Errorf("image-only variant: %w", err)
	}
	gs := "ghostscript"
	if out, err := run(ctx, "gs", "--version"); err == nil {
		gs += " " + strings.TrimSpace(string(out))
	}
	imgSHA, err := c.putVariant(ctx, j, variant{Kind: "image_only_pdf", Input: j.SHA256, Engine: gs, Description: "description.text_removed"}, img)
	if err != nil {
		return results{}, err
	}
	useOurs := settingString(j, "preference", "existing") == "tildoc"
	return recognise(ctx, c, j, img, dir, variant{Kind: "searchable_pdf", Input: imgSHA, Description: "description.full",
		Meta: map[string]any{"langs": langs}}, nil, langs, useOurs)
}

// recognise runs ocrmypdf and uploads the result; ownText: its text becomes the search text.
func recognise(ctx context.Context, c *client, j job, in, dir string, v variant, extra []string, langs string, ownText bool) (results, error) {
	out := filepath.Join(dir, "out.pdf")
	args := append(append([]string{}, extra...), "--output-type", "pdf", "--jobs", "1", "--quiet", "-l", langs, in, out)
	if _, err := run(ctx, "ocrmypdf", args...); err != nil {
		return results{}, err
	}
	text, err := pdfText(ctx, out)
	if err != nil {
		return results{}, err
	}
	v.Engine, v.ProvidesText = ocrEngine(ctx), ownText
	if _, err := c.putVariant(ctx, j, v, out); err != nil {
		return results{}, err
	}
	r := results{Status: "done"}
	if ownText {
		r.Text = &text
	}
	return r, nil
}
