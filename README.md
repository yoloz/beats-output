# syslog

Simple Elastic Beats output to remote syslog plugin;

uses [log/syslog](https://pkg.go.dev/log/syslog)

> 引入对 beats 的依赖`go get github.com/elastic/beats/v7`

## 配置

- address: address of remote syslog collector (string, default: 127.0.0.1:514)
- proto: protocol udp or tcp (string, default: udp)
- see also golang net.Dial documentation
- facility: syslog facility (string, default SYSLOG)
- severity: syslog severity (string, default INFO)

样例:

```yaml
filebeat.inputs:
  - type: log
    ignore_older: 2h
    paths:
      - /var/log/*.log
      - /var/log/syslog
#output.console:
#  pretty: true
output.syslog:
# default 127.0.0.1:514
  address: "127.0.0.1:514"
  # default info
  severity: "INFO"
  facility: "SYSLOG" # default syslog
  proto: "udp" # default udp
  codec.format:
	 string: "%{[message]}"
```

## 实现

### 注册及初始化

在某个 init 函数中注册插件

```go
func init() {
	outputs.RegisterType("syslog", makeSyslog)
}
```

在`makeSyslog`中读取配置生成 syslog 实例

```go
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
```

## 实现输出接口

实现 close()、publish()、string():

```go
// Close close syslog writer.
func (out *syslogOutput) Close() error {
	return out.writer.Close()
}

// Publish sends events to the clients sink.
func (out *syslogOutput) Publish(_ context.Context, batch publisher.Batch) error {
	// ....

	return nil
}

func (out *syslogOutput) String() string {
	return "syslog(" + out.proto + "://" + out.address + ")"
}

```

# 参考文献

- [Writing a Filebeat output plugin](https://www.fullstory.com/blog/writing-a-filebeat-output-plugin)
- [beats-output-remote-syslog](https://github.com/remil1000/beats-output-remote-syslog)

## Generating Beat

在 beats 源码目录中添加 publisher：`/{localpath}/beats/libbeat/publisher/includes/includes.go`

```go
package includes

import (
	// import queue types
	_ "github.com/elastic/beats/v7/libbeat/outputs/codec/format"
	_ "github.com/elastic/beats/v7/libbeat/outputs/codec/json"
	_ "github.com/elastic/beats/v7/libbeat/outputs/console"
	_ "github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
	_ "github.com/elastic/beats/v7/libbeat/outputs/fileout"
	_ "github.com/elastic/beats/v7/libbeat/outputs/kafka"
	_ "github.com/elastic/beats/v7/libbeat/outputs/logstash"
	_ "github.com/elastic/beats/v7/libbeat/outputs/redis"
	_ "github.com/yoloz/beats-output/syslog"   // Register syslog output
	_ "github.com/elastic/beats/v7/libbeat/publisher/queue/diskqueue"
	_ "github.com/elastic/beats/v7/libbeat/publisher/queue/memqueue"
	_ "github.com/elastic/beats/v7/libbeat/publisher/queue/spool"
)
```

:::caution
配合本地开发的 7.10.2 版本的 beats,go.mod 中使用 beats 源码里的依赖
:::
