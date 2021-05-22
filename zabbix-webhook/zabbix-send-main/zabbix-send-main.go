package main

import (
	"fmt"
	"time"

	. "mpk.lcl/zabbix-webhook/modules/zbx"
)

const (
	defaultHost = `172.16.0.130`
	defaultPort = 10051
)

func main() {
	var metrics []*Metric
	metrics = append(metrics, NewMetric("test", "cpu", "100.25", time.Now().Unix()))
	metrics = append(metrics, NewMetric("test", "status", "OK"))
	// Create instance of Packet class
	packet := NewPacket(metrics)
	// Send packet to zabbix
	z := NewSender(defaultHost, defaultPort)
	z.Send(packet)
	fmt.Printf("send ok 6")
}
