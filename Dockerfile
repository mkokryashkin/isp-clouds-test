# syntax=docker/dockerfile:1

FROM golang:1.16-alpine

RUN apk add  --no-cache ffmpeg

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /server

EXPOSE 8080

CMD [ "/server" ]
