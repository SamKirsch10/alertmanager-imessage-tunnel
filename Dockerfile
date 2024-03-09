FROM golang:1.22.1 as builder

COPY . .
RUN echo $(pwd) && go build .


FROM ubuntu

COPY --from=builder /go/alertmanager-imessage-tunnel /app

ENTRYPOINT [ "/app" ]
