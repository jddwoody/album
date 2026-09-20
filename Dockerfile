FROM golang:1.25-alpine3.22 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
COPY internal/ ./internal
COPY config/* ./

RUN go build

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ffmpeg \
	&& addgroup -S albums \
	&& adduser -S -G albums albums \
	&& mkdir /app/logs \
	&& chown -R albums:albums /app

COPY --from=build --chown=albums:albums /app/album /app/album
COPY --from=build --chown=albums:albums /app/start_album.sh /app/start_album.sh
COPY --from=build --chown=albums:albums /app/appconfig.yaml /app/appconfig.yaml

USER albums

ENTRYPOINT [ "./start_album.sh" ]

