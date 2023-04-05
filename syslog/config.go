package syslog

import (
	"fmt"
	"log/syslog"
)

type syslogConfig struct {
	// Number of worker goroutines publishing log events
	workers int `config:"workers" validate:"min=1"`
	// Max number of events in a batch to send to a single client
	batchSize int `config:"batch_size" validate:"min=1"`
	// Max number of retries for single batch of events
	retryLimit int `config:"retry_limit"`
	// protocol of network,like tcp or udp
	protocol string `config:"protocol"`
	// syslog server address
	address string `config:"address"`
	// The Priority is a combination of the syslog facility and severity.
	priority int `config:"priority"`
	//
	tag string `config:"tag"`
}

var (
	defaultConfig = syslogConfig{
		//Workers:  1,
		protocol: "tcp",
		address:  "",
		priority: int(syslog.LOG_INFO),
		tag:      "",
	}
)

func (c *syslogConfig) Validate() error {
	if c.address == "" {
		return fmt.Errorf("%s", "Address is empty")
	}
	return nil
}
