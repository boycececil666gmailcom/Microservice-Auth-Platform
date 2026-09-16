FROM golang:1.26.6-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/shortener ./cmd/shortener

FROM alpine:3.22
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 app
COPY --from=build /out/shortener /usr/local/bin/shortener
USER app
EXPOSE 8001
ENTRYPOINT ["/usr/local/bin/shortener"]
