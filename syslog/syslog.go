// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package syslog

import (
	"context"
	"log/syslog"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/outputs/codec"

	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/publisher"
)

type syslogOutput struct {
	address  string
	proto    string
	log      *logp.Logger
	writer   *syslog.Writer
	beat     beat.Info
	observer outputs.Observer
	codec    codec.Codec
}

func init() {
	outputs.RegisterType("syslog", makeSyslog)
}

// MakeSyslog instantiates a new syslog output instance.
func makeSyslog(_ outputs.IndexManager, beat beat.Info, observer outputs.Observer, cfg *common.Config) (outputs.Group, error) {
	config := defaultConfig
	if err := cfg.Unpack(&config); err != nil {
		return outputs.Fail(err)
	}

	// disable bulk support in publisher pipeline
	cfg.SetInt("bulk_max_size", -1, -1)

	fo := &syslogOutput{
		beat:     beat,
		observer: observer,
		log:      logp.NewLogger("syslog"),
	}
	if err := fo.init(beat, config); err != nil {
		return outputs.Fail(err)
	}

	return outputs.Success(-1, 0, fo)
}

func (out *syslogOutput) init(beat beat.Info, c config) error {
	out.address = c.Address
	out.proto = c.Proto

	severity, err := SeverityPriority(c.Severity)
	if err != nil {
		return err
	}
	facility, err := FacilityPriority(c.Facility)
	if err != nil {
		return err
	}

	out.writer, err = syslog.Dial(out.proto, out.address, severity|facility, "filebeat-syslog")
	if err != nil {
		return err
	}

	out.codec, err = codec.CreateEncoder(beat, c.Codec)
	if err != nil {
		return err
	}

	out.log.Infof("Initialized syslog output. proto=%v address=%v severity=%v facility=%v",
		out.proto, out.address, c.Severity, c.Facility)

	return nil
}

// Implement Outputer:close,publish,string

// Close close syslog writer.
func (out *syslogOutput) Close() error {
	return out.writer.Close()
}

// Publish sends events to the clients sink.
func (out *syslogOutput) Publish(_ context.Context, batch publisher.Batch) error {
	defer batch.ACK()

	st := out.observer
	events := batch.Events()
	st.NewBatch(len(events))

	dropped := 0
	for i := range events {
		event := &events[i]

		serializedEvent, err := out.codec.Encode(out.beat.Beat, &event.Content)
		if err != nil {
			if event.Guaranteed() {
				out.log.Errorf("Failed to serialize[%v] the event: %v", event.Content, err)
			} else {
				out.log.Warnf("Failed to serialize[%v] the event: %v", event.Content, err)
			}

			dropped++
			continue
		}

		if _, err = out.writer.Write(append(serializedEvent, '\n')); err != nil {
			st.WriteError(err)

			if event.Guaranteed() {
				out.log.Errorf("Sending event to remote syslog failed with: %v", err)
			} else {
				out.log.Warnf("Sending event to remote syslog failed with: %v", err)
			}

			dropped++
			continue
		}

		st.WriteBytes(len(serializedEvent) + 1)
	}

	st.Dropped(dropped)
	st.Acked(len(events) - dropped)

	return nil
}

func (out *syslogOutput) String() string {
	return "syslog(" + out.proto + "://" + out.address + ")"
}
