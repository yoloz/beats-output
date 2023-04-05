package syslog

import (
	"context"
	"encoding/json"
	"fmt"
	"log/syslog"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/publisher"
)

// Client struct
type logClient struct {
	// protocol of network,like tcp or udp
	protocol string `config:"protocol"`
	// syslog server address
	address string `config:"address"`
	// The Priority is a combination of the syslog facility and severity.
	priority int `config:"priority"`
	//
	tag string `config:"tag"`

	stats  outputs.Observer
	writer syslog.Writer
}

type eventRaw map[string]json.RawMessage

type event struct {
	Timestamp time.Time     `json:"@timestamp"`
	Fields    common.MapStr `json:"-"`
}

func (c *logClient) Priority() syslog.Priority {
	switch c.priority {
	case 0:
		return syslog.LOG_EMERG
	case 1:
		return syslog.LOG_ALERT
	case 2:
		return syslog.LOG_CRIT
	case 3:
		return syslog.LOG_ERR
	case 4:
		return syslog.LOG_WARNING
	case 5:
		return syslog.LOG_NOTICE
	case 6:
		return syslog.LOG_INFO
	case 7:
		return syslog.LOG_DEBUG
	}
	return syslog.LOG_INFO
}

// Connect establishes a connection to the clients sink.
func (c *logClient) Connect() error {
	logwriter, err := syslog.Dial(c.protocol, c.address, c.Priority(), c.tag)
	if err == nil {
		c.writer = *logwriter
	}
	return err
}

// Close closes a connection.
func (c *logClient) Close() error {
	return c.writer.Close()
}

func (c *logClient) String() string {
	return "syslog"
}

// Publish sends events to the clients sink.
func (c *logClient) Publish(_ context.Context, batch publisher.Batch) error {
	events := batch.Events()
	// 记录这批日志
	c.stats.NewBatch(len(events))
	failEvents, err := c.publishEvents(events)
	if len(failEvents) == 0 {
		batch.ACK()
	} else {
		batch.RetryEvents(failEvents)
	}
	return err
}

// PublishEvents posts all events to the http endpoint. On error a slice with all
// events not published will be returned.
func (c *logClient) publishEvents(events []publisher.Event) ([]publisher.Event, error) {
	if len(events) == 0 {
		return nil, nil
	}
	for i, event := range events {
		err := c.PublishEvent(event)
		if err != nil {
			// 如果单条消息发送失败，则将剩余的消息直接重试
			return events[i:], err
		}
	}
	return nil, nil
}

// PublishEvent publish a single event to output.
func (c *logClient) PublishEvent(event publisher.Event) error {
	logger.Debugf("Publish event: %s", event)
	bytes, err := json.Marshal(&event.Content)
	if err != nil {
		// 如果编码失败，就不重试了，重试也不会成功
		// encode error, don't retry.
		// consider being success
		return nil
	}
	fmt.Fprintf(&c.writer, string(bytes))
	return nil
}

//this should ideally be in enc.go
func makeEvent(v *beat.Event) map[string]json.RawMessage {
	// Inline not supported,
	// HT: https://stackoverflow.com/questions/49901287/embed-mapstringstring-in-go-json-marshaling-without-extra-json-property-inlin
	type event0 event // prevent recursion
	e := event{Timestamp: v.Timestamp.UTC(), Fields: v.Fields}
	b, err := json.Marshal(event0(e))
	if err != nil {
		logger.Warn("Error encoding event to JSON: %v", err)
	}

	var eventMap map[string]json.RawMessage
	err = json.Unmarshal(b, &eventMap)
	if err != nil {
		logger.Warn("Error decoding JSON to map: %v", err)
	}
	// Add the individual fields to the map, flatten "Fields"
	for j, k := range e.Fields {
		b, err = json.Marshal(k)
		if err != nil {
			logger.Warn("Error encoding map to JSON: %v", err)
		}
		eventMap[j] = b
	}
	return eventMap
}
