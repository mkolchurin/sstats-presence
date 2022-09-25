# syntax=docker/dockerfile:1

FROM golang:latest
RUN mkdir /app
ADD . /app/
WORKDIR /app
EXPOSE 8080
EXPOSE 8081
RUN go build -o main .
CMD ["/app/main"]