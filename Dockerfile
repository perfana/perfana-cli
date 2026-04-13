FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY perfana-cli /usr/local/bin/perfana-cli

ENTRYPOINT ["perfana-cli"]
