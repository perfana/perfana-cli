FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/perfana-cli .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 perfana && adduser -D -u 1000 -G perfana perfana

COPY --from=builder /out/perfana-cli /usr/local/bin/perfana-cli

USER perfana

ENTRYPOINT ["perfana-cli"]
