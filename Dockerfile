# tildoc-ocr: text extraction and OCR, talking to tildoc over REST.
# OCR_LANGS decides which languages exist (Tesseract codes, comma separated).
FROM golang:1.27-trixie AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tildoc-ocr .

FROM debian:trixie-slim AS tesseract
ARG TESSERACT_VERSION=5.5.4
ARG TESSERACT_SHA256=a8dbf8b8c8ee427ff6ad1d99116502feaf59440af1adb893caf60a6576826cf4
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl build-essential autoconf automake libtool pkg-config libleptonica-dev libarchive-dev libcurl4-openssl-dev zlib1g-dev
RUN curl -fsSL -o /tmp/tesseract.tar.gz "https://github.com/tesseract-ocr/tesseract/archive/refs/tags/${TESSERACT_VERSION}.tar.gz" \
 && echo "${TESSERACT_SHA256}  /tmp/tesseract.tar.gz" | sha256sum -c - \
 && tar -xzf /tmp/tesseract.tar.gz -C /tmp \
 && cd "/tmp/tesseract-${TESSERACT_VERSION}" \
 && ./autogen.sh && ./configure --prefix=/usr/local --disable-openmp --without-curl \
 && make -j"$(nproc)" && make install \
 && rm -rf /tmp/tesseract* /usr/local/share/tessdata/*.traineddata

FROM debian:trixie-slim
ARG OCR_LANGS=eng,deu
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates poppler-utils ocrmypdf ghostscript libleptonica6 libarchive13t64 libgomp1 \
      $(echo "$OCR_LANGS" | tr ',' '\n' | sed 's/^/tesseract-ocr-/' | tr '\n' ' ') \
 && rm -rf /var/lib/apt/lists/*
COPY --from=tesseract /usr/local/ /usr/local/
COPY --from=build /out/tildoc-ocr /usr/local/bin/tildoc-ocr
RUN ldconfig && tesseract --version | head -1 && useradd -r -m tildococr
ENV TESSDATA_PREFIX=/usr/share/tesseract-ocr/5/tessdata OCR_LANGS=$OCR_LANGS
USER tildococr
CMD ["tildoc-ocr"]
