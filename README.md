# tildoc-ocr

OCR for [tildoc](https://github.com/tildoc/tildoc): recognises scanned PDFs with ocrmypdf and Tesseract and adds a searchable PDF as a variant. Reading text is left to [tildoc-text](https://github.com/tildoc/tildoc-text), including the text of that variant.

It talks to tildoc only through the plugin REST API and needs no database and no master key.

## Environment

| Variable | Meaning |
|---|---|
| `PLUGIN_LISTEN` | Where it takes jobs, default `:8000` |
| `TILDOC_URL` | Address of the tildoc server, e.g. `http://server:8080` |
| `PLUGIN_CONCURRENCY` | Files at a time, default 2 |

## Languages

Which languages exist is fixed when the image is built, and the Administration page offers them:

```
docker build --build-arg OCR_LANGS=eng,deu .
```

Licence: GNU AGPL-3.0-only, like tildoc.

The program holds no credential. tildoc hands it each job with the file and a token that can only deliver the result of that job.
