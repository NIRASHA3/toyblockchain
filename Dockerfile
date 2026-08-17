FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/toychain ./cmd/toychain

FROM alpine:3.20

RUN adduser -D -u 10001 toychain \
    && mkdir -p /app /data \
    && chown -R toychain:toychain /app /data

COPY --from=build /out/toychain /usr/local/bin/toychain

USER toychain
WORKDIR /app

VOLUME ["/data"]

EXPOSE 8081 8082 8083

CMD ["toychain", "help"]