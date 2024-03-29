FROM golang:1.21.6-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY . /app
RUN go mod download
RUN go build ./cmd/alterx

FROM alpine:3.19.0
RUN apk -U upgrade --no-cache \
    && apk add --no-cache bind-tools ca-certificates
COPY --from=builder /app/alterx /usr/local/bin/

ENTRYPOINT ["alterx"]
