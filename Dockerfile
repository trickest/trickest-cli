FROM --platform=linux/amd64 golang:1.17.1-alpine as build

RUN apk update

COPY . /app

WORKDIR /app

RUN env GOOS=linux GOARCH=amd64 go build .

FROM --platform=linux/amd64 alpine:3.16.0

COPY --from=build  /app/trickest-cli /usr/bin/trickest

ENTRYPOINT ["trickest"]
