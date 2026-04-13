FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 perfana && adduser -D -u 1000 -G perfana perfana

COPY perfana-cli /usr/local/bin/perfana-cli

USER perfana

ENTRYPOINT ["perfana-cli"]
