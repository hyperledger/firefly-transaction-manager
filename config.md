---
layout: default
title: Configuration Reference
parent: Reference
nav_order: 3
---

# Configuration Reference
{: .no_toc }

<!-- ## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc} -->

---


## api

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|address|Listener address for API|`string`|`127.0.0.1`
|port|Listener port for API|`int`|`5008`
|publicURL|External address callers should access API over|`string`|`<nil>`
|readTimeout|The maximum time to wait when reading from an HTTP connection|[`time.Duration`](https://pkg.go.dev/time#Duration)|`15s`
|shutdownTimeout|The maximum amount of time to wait for any open HTTP requests to finish before shutting down the HTTP server|[`time.Duration`](https://pkg.go.dev/time#Duration)|`10s`
|writeTimeout|The maximum time to wait when writing to a HTTP connection|[`time.Duration`](https://pkg.go.dev/time#Duration)|`15s`

## api.tls

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|caFile|The path to the CA file for TLS on this API|`string`|`<nil>`
|certFile|The path to the certificate file for TLS on this API|`string`|`<nil>`
|clientAuth|Enables or disables client auth for TLS on this API|`string`|`<nil>`
|enabled|Enables or disables TLS on this API|`boolean`|`false`
|keyFile|The path to the private key file for TLS on this API|`string`|`<nil>`

## confirmations

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|blockCacheSize|The maximum number of block headers to keep in the cache|`int`|`1000`
|blockPollingInterval|How often to poll for new block headers|[`time.Duration`](https://pkg.go.dev/time#Duration)|`3s`
|notificationQueueLength|Internal queue length for notifying the confirmations manager of new transactions/events|`int`|`50`
|required|Number of confirmations required to consider a transaction/event final|`int`|`20`
|staleReceiptTimeout|Duration after which to force a receipt check for a pending transaction|[`time.Duration`](https://pkg.go.dev/time#Duration)|`1m`

## connector

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|connectionTimeout|The maximum amount of time that a connection is allowed to remain with no data transmitted|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|expectContinueTimeout|See [ExpectContinueTimeout in the Go docs](https://pkg.go.dev/net/http#Transport)|[`time.Duration`](https://pkg.go.dev/time#Duration)|`1s`
|headers|Adds custom headers to HTTP requests|`map[string]string`|`<nil>`
|idleTimeout|The max duration to hold a HTTP keepalive connection between calls|[`time.Duration`](https://pkg.go.dev/time#Duration)|`475ms`
|maxIdleConns|The max number of idle connections to hold pooled|`int`|`100`
|requestTimeout|The maximum amount of time that a request is allowed to remain open|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|tlsHandshakeTimeout|The maximum amount of time to wait for a successful TLS handshake|[`time.Duration`](https://pkg.go.dev/time#Duration)|`10s`
|url|The URL of the blockchain connector|`string`|`<nil>`
|variant|The variant is the overall category of blockchain connector, defining things like how input/output definitions are passed|`string`|`evm`

## connector.auth

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|password|Password|`string`|`<nil>`
|username|Username|`string`|`<nil>`

## connector.proxy

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|url|Optional HTTP proxy URL to use for the blockchain connector|`string`|`<nil>`

## connector.retry

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|count|The maximum number of times to retry|`int`|`5`
|enabled|Enables retries|`boolean`|`false`
|initWaitTime|The initial retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`250ms`
|maxWaitTime|The maximum retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`

## cors

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|credentials|CORS setting to control whether a browser allows credentials to be sent to this API|`boolean`|`true`
|debug|Whether debug is enabled for the CORS implementation|`boolean`|`false`
|enabled|Whether CORS is enabled|`boolean`|`true`
|headers|CORS setting to control the allowed headers|`string`|`[*]`
|maxAge|The maximum age a browser should rely on CORS checks|[`time.Duration`](https://pkg.go.dev/time#Duration)|`600`
|methods| CORS setting to control the allowed methods|`string`|`[GET POST PUT PATCH DELETE]`
|origins|CORS setting to control the allowed origins|`string`|`[*]`

## ffcore

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|connectionTimeout|The maximum amount of time that a connection is allowed to remain with no data transmitted|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|expectContinueTimeout|See [ExpectContinueTimeout in the Go docs](https://pkg.go.dev/net/http#Transport)|[`time.Duration`](https://pkg.go.dev/time#Duration)|`1s`
|headers|Adds custom headers to HTTP requests|`map[string]string`|`<nil>`
|idleTimeout|The max duration to hold a HTTP keepalive connection between calls|[`time.Duration`](https://pkg.go.dev/time#Duration)|`475ms`
|maxIdleConns|The max number of idle connections to hold pooled|`int`|`100`
|requestTimeout|The maximum amount of time that a request is allowed to remain open|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|tlsHandshakeTimeout|The maximum amount of time to wait for a successful TLS handshake|[`time.Duration`](https://pkg.go.dev/time#Duration)|`10s`
|url|The URL of the FireFly core admin API server to connect to|`string`|`<nil>`

## ffcore.auth

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|password|Password|`string`|`<nil>`
|username|Username|`string`|`<nil>`

## ffcore.proxy

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|url|Optional HTTP proxy URL to use for the FireFly core admin API server|`string`|`<nil>`

## ffcore.retry

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|count|The maximum number of times to retry|`int`|`5`
|enabled|Enables retries|`boolean`|`false`
|initWaitTime|The initial retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`250ms`
|maxWaitTime|The maximum retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`

## ffcore.ws

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|heartbeatInterval|The amount of time to wait between heartbeat signals on the WebSocket connection|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|initialConnectAttempts|The number of attempts FireFly will make to connect to the WebSocket when starting up, before failing|`int`|`5`
|path|The WebSocket sever URL to which FireFly should connect|WebSocket URL `string`|`/admin/ws`
|readBufferSize|The size in bytes of the read buffer for the WebSocket connection|[`BytesSize`](https://pkg.go.dev/github.com/docker/go-units#BytesSize)|`16Kb`
|writeBufferSize|The size in bytes of the write buffer for the WebSocket connection|[`BytesSize`](https://pkg.go.dev/github.com/docker/go-units#BytesSize)|`16Kb`

## log

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|compress|Determines if the rotated log files should be compressed using gzip|`boolean`|`<nil>`
|filename|Filename is the file to write logs to.  Backup log files will be retained in the same directory|`string`|`<nil>`
|filesize|MaxSize is the maximum size the log file before it gets rotated|[`BytesSize`](https://pkg.go.dev/github.com/docker/go-units#BytesSize)|`100m`
|forceColor|Force color to be enabled, even when a non-TTY output is detected|`boolean`|`<nil>`
|includeCodeInfo|Enables the report caller for including the calling file and line number, and the calling function. If using text logs, it uses the logrus text format rather than the default prefix format.|`boolean`|`false`
|level|The log level - error, warn, info, debug, trace|`string`|`info`
|maxAge|The maximum time to retain old log files based on the timestamp encoded in their filename.|[`time.Duration`](https://pkg.go.dev/time#Duration)|`24h`
|maxBackups|Maximum number of old log files to retain|`int`|`2`
|noColor|Force color to be disabled, event when TTY output is detected|`boolean`|`<nil>`
|timeFormat|Custom time format for logs|[Time format](https://pkg.go.dev/time#pkg-constants) `string`|`2006-01-02T15:04:05.000Z07:00`
|utc|Use UTC timestamps for logs|`boolean`|`false`

## log.json

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|enabled|Enables JSON formatted logs rather than text. All log color settings are ignored when enabled.|`boolean`|`false`

## log.json.fields

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|file|configures the JSON key containing the calling file|`string`|`file`
|func|Configures the JSON key containing the calling function|`string`|`func`
|level|Configures the JSON key containing the log level|`string`|`level`
|message|Configures the JSON key containing the log message|`string`|`message`
|timestamp|Configures the JSON key containing the timestamp of the log|`string`|`@timestamp`

## manager

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|name|The name of this Transaction Manager, used in operation metadata to track which operations are to be updated|`string`|`<nil>`

## operations

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|errorHistoryCount|The number of historical errors to retain in the operation|`int`|`25`
|types|The operation types to query in FireFly core, that might have been submitted via this Transaction Manager|string[]|`[blockchain_invoke blockchain_pin_batch token_create_pool]`

## operations.changeListener

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|enabled|Whether to enable the change event listener to detect updates made to operations outside of the FFTM|`boolean`|`<nil>`

## operations.fullScan

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|minimumDelay|The minimum delay between full scans of the FireFly core API, when reconnecting, or recovering from missed events / errors|[`time.Duration`](https://pkg.go.dev/time#Duration)|`5s`
|pageSize|The page size to use when performing a full scan of the ForeFly core API on startup, or recovery|`int`|`100`
|startupMaxRetries|The page size to use when performing a full scan of the ForeFly core API on startup, or recovery|`int`|`10`

## policyengine

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|name|The name of the policy engine to use|`string`|`simple`

## policyengine.simple

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|fixedGasPrice|A fixed gasPrice value/structure to pass to the connector|Raw JSON|`<nil>`
|warnInterval|The time between warnings when a blockchain transaction has not been allocated a receipt|[`time.Duration`](https://pkg.go.dev/time#Duration)|`15m`

## policyengine.simple.gasOracle

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|connectionTimeout|The maximum amount of time that a connection is allowed to remain with no data transmitted|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|expectContinueTimeout|See [ExpectContinueTimeout in the Go docs](https://pkg.go.dev/net/http#Transport)|[`time.Duration`](https://pkg.go.dev/time#Duration)|`1s`
|headers|Adds custom headers to HTTP requests|`map[string]string`|`<nil>`
|idleTimeout|The max duration to hold a HTTP keepalive connection between calls|[`time.Duration`](https://pkg.go.dev/time#Duration)|`475ms`
|maxIdleConns|The max number of idle connections to hold pooled|`int`|`100`
|method|The HTTP Method to use when invoking the Gas Oracle REST API|`string`|`GET`
|mode|The gas oracle mode|connector | restapi | disabled|`disabled`
|queryInterval|The minimum interval between queries to the Gas Oracle|[`time.Duration`](https://pkg.go.dev/time#Duration)|`5m`
|requestTimeout|The maximum amount of time that a request is allowed to remain open|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|template|REST API Gas Oracle: A go template to execute against the result from the Gas Oracle, to create a JSON block that will be passed as the gas price to the connector|[Go Template](https://pkg.go.dev/text/template) `string`|`<nil>`
|tlsHandshakeTimeout|The maximum amount of time to wait for a successful TLS handshake|[`time.Duration`](https://pkg.go.dev/time#Duration)|`10s`
|url|REST API Gas Oracle: The URL of a Gas Oracle REST API to call|`string`|`<nil>`

## policyengine.simple.gasOracle.auth

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|password|Password|`string`|`<nil>`
|username|Username|`string`|`<nil>`

## policyengine.simple.gasOracle.proxy

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|url|Optional HTTP proxy URL to use for the Gas Oracle REST API|`string`|`<nil>`

## policyengine.simple.gasOracle.retry

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|count|The maximum number of times to retry|`int`|`5`
|enabled|Enables retries|`boolean`|`false`
|initWaitTime|The initial retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`250ms`
|maxWaitTime|The maximum retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`

## policyloop

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|interval|Interval at which to invoke the policy engine to evaluate outstanding transactions|[`time.Duration`](https://pkg.go.dev/time#Duration)|`1s`