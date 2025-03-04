FROM golang:1.19 AS build

ENV CGO_ENABLED=0
ENV GOOS=linux

WORKDIR /go/src/sbpemulator

# Update dependencies
COPY go.* ./
RUN go mod download

# Build
COPY . ./
RUN go build -o ./app/spbemulator ./cmd/sparkplugb/

# Pack
FROM busybox AS package

WORKDIR /

COPY --from=build /go/src/sbpemulator/app/spbemulator .
COPY ./configs/config.json /configs/config.json

EXPOSE 8080

ENTRYPOINT ["/spbemulator"]
