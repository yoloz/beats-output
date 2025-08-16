# beats-output

more output for beats

## syslog

Simple Elastic Beats syslog output;

## 实现

### 注册及初始化

- 在`init`函数中注册插件
- 在`makeSyslog`函数中读取配置生成 syslog 实例

### 实现输出接口

实现 close()、publish()、string():

### 测试

```go
go test ./syslog -run Test -v
```

### Generating Beat

在 beats 源码目录中添加 publisher：`/{localpath}/beats/libbeat/publisher/includes/includes.go`

```go
package includes

import (
	//...
	_ "github.com/yoloz/beats-output/syslog"
)
```
