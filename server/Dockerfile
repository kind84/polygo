FROM dependencies AS builder

WORKDIR /polygo/server

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
go build -o /go/bin/server /polygo/server/

ENTRYPOINT ["/go/bin/server"]


FROM alpine:latest

ENV POLYGO_SERVER_PORT=8080
COPY --from=builder /go/bin/server /bin/server

ENTRYPOINT ["/bin/server"]
