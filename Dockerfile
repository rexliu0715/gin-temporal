ARG GO_VERSION=latest

FROM golang:${GO_VERSION}-alpine AS builder

RUN apk update && apk add alpine-sdk git && rm -rf /var/cache/apk/*

RUN mkdir -p /app
WORKDIR /app

COPY go.mod .
RUN go mod download

COPY . .
RUN go build -o ./server ./main.go

FROM alpine:latest

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

RUN mkdir -p /app
WORKDIR /app

COPY --from=builder /app/server .

WORKDIR /app

EXPOSE 8088

ENTRYPOINT ["./server"]
