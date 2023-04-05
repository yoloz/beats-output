package syslog

import (
	"errors"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/outputs"
)

func init() {
	outputs.RegisterType("syslog", newWsOutput)
}

var (
	logger = logp.NewLogger("output.syslog")
	// ErrNotConnected indicates failure due to client having no valid connection
	ErrNotConnected = errors.New("not connected")
	// ErrJSONEncodeFailed indicates encoding failures
	ErrJSONEncodeFailed = errors.New("json encode failed")
)

func newWsOutput(_ outputs.IndexManager, _ beat.Info, stats outputs.Observer, cfg *common.Config) (outputs.Group, error) {
	config := defaultConfig
	// 卸载配置
	if err := cfg.Unpack(&config); err != nil {
		return outputs.Fail(err)
	}
	clients := make([]outputs.NetworkClient, config.workers)
	for i := 0; i < config.workers; i++ {
		logger.Info("Making client for addr: " + config.address)
		clients[i] = &logClient{
			protocol: config.protocol,
			address:  config.address,
			priority: config.priority,
			tag:      config.tag,
			stats:    stats,
		}
	}

	return outputs.SuccessNet(true, config.batchSize, config.retryLimit, clients)
}
