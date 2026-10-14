FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/url-shortener ./cmd/url-shortener

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /bin/url-shortener /app/url-shortener
COPY config ./config

EXPOSE 8082

ENTRYPOINT ["/app/url-shortener"]
