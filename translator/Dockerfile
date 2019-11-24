FROM dependencies AS builder

WORKDIR /polygo/translator

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
go build -o /go/bin/translator /polygo/translator/cmd/translator

ENTRYPOINT ["/go/bin/translator"]


FROM alpine:latest

COPY --from=builder /go/bin/translator /bin/translator

ENV GOOGLE_APPLICATION_CREDENTIALS="/gcloud-key.json"

ENTRYPOINT ["/bin/translator"]
