# tildoc-ocr

Text extraction and OCR for [tildoc](https://github.com/tildoc/tildoc), as two plugins in one program:

| Plugin | Does |
|---|---|
| `tildoc-text` | Text of plain-text files and text embedded in PDFs |
| `tildoc-ocr` | Recognises scanned pages with ocrmypdf and Tesseract, adds a searchable PDF |

It talks to tildoc only through the plugin REST API. It needs no database and no master key.

## Environment

| Variable | Meaning |
|---|---|
| `TILDOC_URL` | Address of the tildoc server, e.g. `http://server:8080` |
| `TILDOC_TEXT_TOKEN`, `TILDOC_OCR_TOKEN` | The tokens from the plugin manifests |
| `TILDOC_OCR_CONCURRENCY` | Files at a time per plugin, default 2 |

## Languages

Which languages exist is fixed when the image is built, and the Administration page offers them:

```
docker build --build-arg OCR_LANGS=eng,deu .
```

Licence: GNU AGPL-3.0-only, like tildoc.
