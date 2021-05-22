# Build
FROM golang:1 as build

WORKDIR /go/src/zabbix-webhook/zabbix-webhook-main

COPY ./zabbix-webhook  /go/src/zabbix-webhook

RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o alertmanager-zabbix-webhook .

# Run
FROM alpine:latest

RUN adduser webhook -s /bin/false -D webhook

RUN mkdir -p /etc/webhook
COPY ./zabbix-webhook/zabbix-webhook-main/config.yaml /etc/webhook

COPY --from=build /go/src/zabbix-webhook/zabbix-webhook-main/alertmanager-zabbix-webhook /usr/bin

EXPOSE 8081
USER webhook

ENTRYPOINT ["/usr/bin/alertmanager-zabbix-webhook"]
CMD ["-config", "/etc/webhook/config.yaml"]
