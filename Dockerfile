FROM golang:1.20-alpine3.19

WORKDIR /app

RUN mkdir logs
COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
ADD internal/ ./internal
COPY config/* ./

RUN go build

RUN apk update
RUN apk upgrade
RUN apk add --no-cache ffmpeg
RUN apk add --no-cache bash
RUN apk add --no-cache mlocate

ENTRYPOINT [ "./start_album.sh" ]

