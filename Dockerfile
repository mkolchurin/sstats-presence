# syntax=docker/dockerfile:1
FROM golang:alpine AS builder

WORKDIR /build

ADD go.mod .

COPY . .

RUN go build -o sstats-presence main.go

FROM alpine

WORKDIR /build
EXPOSE 8081
COPY --from=builder /build/sstats-presence /build/sstats-presence

CMD ["./sstats-presence"]