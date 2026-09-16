FROM golang:1.26.6-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/analytics ./cmd/analytics

FROM alpine:3.22
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 app
COPY --from=build /out/analytics /usr/local/bin/analytics
USER app
EXPOSE 8003
ENTRYPOINT ["/usr/local/bin/analytics"]
