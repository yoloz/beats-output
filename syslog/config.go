package syslog

// config holds configuration for the syslog output.
type config struct {
	// Network defines the transport for remote syslog: "udp" or "tcp".
	// If empty, the output will try to write to the local syslog.
	Network string `config:"network"`
	// Host is the remote address in host:port format for network transports.
	Host string `config:"host"`

	// Facility used to compute PRI in RFC messages (default: "user").
	Facility string `config:"facility"` // e.g. "daemon", "user"
	// Severity used for messages when mapping to PRI (default: "info").
	Severity string `config:"severity"`

	// Tag used as APP-NAME in RFC5424 or TAG in RFC3164 (default: "beats").
	Tag string `config:"tag"`

	// Format can be "rfc3164" or "rfc5424". If unset, RFC3164 is used.
	Format string `config:"format"`

	BatchSize int `config:"batch_size"`
	Retry     int `config:"retry"`
	// OnFailure controls what happens when sending fails. "retry" will request
	// the pipeline to retry the batch. "discard" will acknowledge the batch
	// (drop events). Default is "discard".
	OnFailure string `config:"on_failure"`
}

func defaultConfig() config {
	return config{
		Network:   "",
		Host:      "",
		Facility:  "user",
		Severity:  "info",
		Tag:       "beats",
		Format:    "",
		BatchSize: 1000,
		Retry:     3,
		OnFailure: "discard",
	}
}
