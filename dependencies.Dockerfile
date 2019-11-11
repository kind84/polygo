FROM golang:alpine AS dep

ENV GOPROXY=https://proxy.golang.org

WORKDIR /polygo

COPY go.mod .
COPY go.sum .

# RUN apk update && apk add git gcc libc-dev 
RUN go mod download

# Add here shared packages
COPY ./pkg ./pkg
