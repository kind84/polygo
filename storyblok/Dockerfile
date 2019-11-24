FROM dependencies AS builder

WORKDIR /polygo/storyblok

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
go build -o /go/bin/storyblok /polygo/storyblok/cmd/storyblok

ENTRYPOINT ["/go/bin/storyblok"]


FROM alpine:latest

COPY --from=builder /go/bin/storyblok /bin/storyblok

ENTRYPOINT ["/bin/storyblok"]
