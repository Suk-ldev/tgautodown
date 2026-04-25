FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/tgautodown .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOBIN=/out go install github.com/GopeedLab/gopeed/cmd/gopeed@latest

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
RUN mkdir -p /app/data /app/download /app/bin
COPY --from=builder /out/tgautodown /app/tgautodown
COPY --from=builder /out/gopeed /app/bin/gopeed
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/tgautodown /app/bin/gopeed /app/docker-entrypoint.sh

EXPOSE 2020
ENTRYPOINT ["/app/docker-entrypoint.sh"]
