package syslog

import (
	"fmt"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/outputs"
)

// Register output
func init() {
	outputs.RegisterType("syslog", makeSyslog)
}

func makeSyslog(
	im outputs.IndexManager,
	info beat.Info,
	stats outputs.Observer,
	cfg *common.Config,
) (outputs.Group, error) {
	c := defaultConfig()
	if cfg != nil {
		if err := cfg.Unpack(&c); err != nil {
			return outputs.Group{}, fmt.Errorf("failed to unpack syslog output config: %w", err)
		}
	}

	// create the client that writes to local syslog
	client, err := newSyslogClient(c)
	if err != nil {
		return outputs.Group{}, err
	}

	g := outputs.Group{
		Clients:   []outputs.Client{client},
		BatchSize: c.BatchSize,
		Retry:     c.Retry,
	}
	return g, nil
}
