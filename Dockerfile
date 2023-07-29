ARG GO_VERSION=1.20.5

FROM golang:${GO_VERSION}-alpine AS builder

RUN apk update && apk add alpine-sdk git && rm -rf /var/cache/apk/*

RUN mkdir -p /app
WORKDIR /app

COPY src/go.mod .
COPY src/go.sum .
RUN go mod download

COPY src .

RUN go build -o ./main ./main.go

FROM alpine:latest

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

RUN mkdir -p /app

WORKDIR /app

COPY --from=builder /app/main .

COPY src/.env.prod .env

EXPOSE 8088

ENTRYPOINT ["./main"]
