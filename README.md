# tildoc-ocr

OCR for [tildoc](https://github.com/tildoc/tildoc): recognises scanned PDFs with ocrmypdf and Tesseract and adds a searchable PDF as a variant. Reading text is left to [tildoc-text](https://github.com/tildoc/tildoc-text), including the text of that variant.

It talks to tildoc only through the plugin REST API and needs no database and no master key.

## Environment

| Variable | Meaning |
|---|---|
| `TILDOC_URL` | Address of the tildoc server, e.g. `http://server:8080` |
| `TILDOC_OCR_TOKEN` | The token from the plugin manifest |
| `TILDOC_OCR_CONCURRENCY` | Files at a time, default 2 |

## Languages

Which languages exist is fixed when the image is built, and the Administration page offers them:

```
docker build --build-arg OCR_LANGS=eng,deu .
```

Licence: GNU AGPL-3.0-only, like tildoc.
