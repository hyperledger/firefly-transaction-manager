# Configuration reference

## api

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|address|Listener address for API|`string`|`127.0.0.1`
|defaultRequestTimeout|Default server-side request timeout for API calls|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|maxRequestTimeout|Maximum server-side request timeout a caller can request with a Request-Timeout header|[`time.Duration`](https://pkg.go.dev/time#Duration)|`10m`
|port|Listener port for API|`int`|`5008`
|publicURL|External address callers should access API over|`string`|`<nil>`
|readTimeout|The maximum time to wait when reading from an HTTP connection|[`time.Duration`](https://pkg.go.dev/time#Duration)|`15s`
|shutdownTimeout|The maximum amount of time to wait for any open HTTP requests to finish before shutting down the HTTP server|[`time.Duration`](https://pkg.go.dev/time#Duration)|`10s`
|writeTimeout|The maximum time to wait when writing to a HTTP connection|[`time.Duration`](https://pkg.go.dev/time#Duration)|`15s`

## api.auth

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|type|The auth plugin to use for server side authentication of requests|`string`|`<nil>`

## api.auth.basic

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|passwordfile|The path to a .htpasswd file to use for authenticating requests. Passwords should be hashed with bcrypt.|`string`|`<nil>`

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
|blockQueueLength|Internal queue length for notifying the confirmations manager of new blocks|`int`|`50`
|notificationQueueLength|Internal queue length for notifying the confirmations manager of new transactions/events|`int`|`50`
|required|Number of confirmations required to consider a transaction/event final|`int`|`20`
|staleReceiptTimeout|Duration after which to force a receipt check for a pending transaction|[`time.Duration`](https://pkg.go.dev/time#Duration)|`1m`

## cors

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|credentials|CORS setting to control whether a browser allows credentials to be sent to this API|`boolean`|`true`
|debug|Whether debug is enabled for the CORS implementation|`boolean`|`false`
|enabled|Whether CORS is enabled|`boolean`|`true`
|headers|CORS setting to control the allowed headers|`[]string`|`[*]`
|maxAge|The maximum age a browser should rely on CORS checks|[`time.Duration`](https://pkg.go.dev/time#Duration)|`600`
|methods| CORS setting to control the allowed methods|`[]string`|`[GET POST PUT PATCH DELETE]`
|origins|CORS setting to control the allowed origins|`[]string`|`[*]`

## debug

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|port|An HTTP port on which to enable the go debugger|`int`|`-1`

## eventstreams

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|checkpointInterval|Regular interval to write checkpoints for an event stream listener that is not actively detecting/delivering events|[`time.Duration`](https://pkg.go.dev/time#Duration)|`1m`

## eventstreams.defaults

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|batchSize|Default batch size for newly created event streams|`int`|`50`
|batchTimeout|Default batch timeout for newly created event streams|[`time.Duration`](https://pkg.go.dev/time#Duration)|`5s`
|blockedRetryDelay|Default blocked retry delay for newly created event streams|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|errorHandling|Default error handling for newly created event streams|'skip' or 'block'|`block`
|retryTimeout|Default retry timeout for newly created event streams|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|webhookRequestTimeout|Default WebHook request timeout for newly created event streams|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|websocketDistributionMode|Default WebSocket distribution mode for newly created event streams|'load_balance' or 'broadcast'|`load_balance`

## eventstreams.retry

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|factor|Factor to increase the delay by, between each retry|`float32`|`2`
|initialDelay|Initial retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`250ms`
|maxDelay|Maximum delay between retries|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`

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

## metrics

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|address|The IP address on which the metrics HTTP API should listen|`int`|`127.0.0.1`
|enabled|Enables the metrics API|`boolean`|`false`
|path|The path from which to serve the Prometheus metrics|`string`|`/metrics`
|port|The port on which the metrics HTTP API should listen|`int`|`6000`
|publicURL|The fully qualified public URL for the metrics API. This is used for building URLs in HTTP responses and in OpenAPI Spec generation|URL `string`|`<nil>`
|readTimeout|The maximum time to wait when reading from an HTTP connection|[`time.Duration`](https://pkg.go.dev/time#Duration)|`15s`
|shutdownTimeout|The maximum amount of time to wait for any open HTTP requests to finish before shutting down the HTTP server|[`time.Duration`](https://pkg.go.dev/time#Duration)|`10s`
|writeTimeout|The maximum time to wait when writing to an HTTP connection|[`time.Duration`](https://pkg.go.dev/time#Duration)|`15s`

## metrics.auth

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|type|The auth plugin to use for server side authentication of requests|`string`|`<nil>`

## metrics.auth.basic

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|passwordfile|The path to a .htpasswd file to use for authenticating requests. Passwords should be hashed with bcrypt.|`string`|`<nil>`

## metrics.tls

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|caFile|The path to the CA file for TLS on this API|`string`|`<nil>`
|certFile|The path to the certificate file for TLS on this API|`string`|`<nil>`
|clientAuth|Enables or disables client auth for TLS on this API|`string`|`<nil>`
|enabled|Enables or disables TLS on this API|`boolean`|`false`
|keyFile|The path to the private key file for TLS on this API|`string`|`<nil>`

## persistence

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|type|The type of persistence to use|Only 'leveldb' currently supported|`leveldb`

## persistence.leveldb

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|maxHandles|The maximum number of cached file handles LevelDB should keep open|`int`|`100`
|path|The path for the LevelDB persistence directory|`string`|`<nil>`
|syncWrites|Whether to synchronously perform writes to the storage|`boolean`|`false`

## policyengine

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|name|The name of the policy engine to use|`string`|`simple`

## policyengine.simple

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|fixedGasPrice|A fixed gasPrice value/structure to pass to the connector|Raw JSON|`<nil>`
|resubmitInterval|The time between warning and re-sending a transaction (same nonce) when a blockchain transaction has not been allocated a receipt|[`time.Duration`](https://pkg.go.dev/time#Duration)|`<nil>`

## policyengine.simple.gasOracle

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|connectionTimeout|The maximum amount of time that a connection is allowed to remain with no data transmitted|[`time.Duration`](https://pkg.go.dev/time#Duration)|`<nil>`
|expectContinueTimeout|See [ExpectContinueTimeout in the Go docs](https://pkg.go.dev/net/http#Transport)|[`time.Duration`](https://pkg.go.dev/time#Duration)|`<nil>`
|headers|Adds custom headers to HTTP requests|`map[string]string`|`<nil>`
|idleTimeout|The max duration to hold a HTTP keepalive connection between calls|[`time.Duration`](https://pkg.go.dev/time#Duration)|`<nil>`
|maxIdleConns|The max number of idle connections to hold pooled|`int`|`<nil>`
|method|The HTTP Method to use when invoking the Gas Oracle REST API|`string`|`<nil>`
|mode|The gas oracle mode|'connector', 'restapi', 'fixed', or 'disabled'|`<nil>`
|queryInterval|The minimum interval between queries to the Gas Oracle|[`time.Duration`](https://pkg.go.dev/time#Duration)|`<nil>`
|requestTimeout|The maximum amount of time that a request is allowed to remain open|[`time.Duration`](https://pkg.go.dev/time#Duration)|`<nil>`
|template|REST API Gas Oracle: A go template to execute against the result from the Gas Oracle, to create a JSON block that will be passed as the gas price to the connector|[Go Template](https://pkg.go.dev/text/template) `string`|`<nil>`
|tlsHandshakeTimeout|The maximum amount of time to wait for a successful TLS handshake|[`time.Duration`](https://pkg.go.dev/time#Duration)|`<nil>`
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
|count|The maximum number of times to retry|`int`|`<nil>`
|enabled|Enables retries|`boolean`|`<nil>`
|initWaitTime|The initial retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`<nil>`
|maxWaitTime|The maximum retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`<nil>`

## policyloop

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|interval|Interval at which to invoke the policy engine to evaluate outstanding transactions|[`time.Duration`](https://pkg.go.dev/time#Duration)|`10s`

## policyloop.retry

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|factor|The retry backoff factor|`float32`|`2`
|initialDelay|The initial retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`250ms`
|maxDelay|The maximum retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`

## transactions

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|errorHistoryCount|The number of historical errors to retain in the operation|`int`|`25`
|maxInFlight|The maximum number of transactions to have in-flight with the policy engine / blockchain transaction pool|`int`|`100`
|nonceStateTimeout|How old the most recently submitted transaction record in our local state needs to be, before we make a request to the node to query the next nonce for a signing address|[`time.Duration`](https://pkg.go.dev/time#Duration)|`1h`

## webhooks

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|allowPrivateIPs|Whether to allow WebHook URLs that resolve to Private IP address ranges (vs. internet addresses)|`boolean`|`true`
|connectionTimeout|The maximum amount of time that a connection is allowed to remain with no data transmitted|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|expectContinueTimeout|See [ExpectContinueTimeout in the Go docs](https://pkg.go.dev/net/http#Transport)|[`time.Duration`](https://pkg.go.dev/time#Duration)|`1s`
|headers|Adds custom headers to HTTP requests|`map[string]string`|`<nil>`
|idleTimeout|The max duration to hold a HTTP keepalive connection between calls|[`time.Duration`](https://pkg.go.dev/time#Duration)|`475ms`
|maxIdleConns|The max number of idle connections to hold pooled|`int`|`100`
|requestTimeout|The maximum amount of time that a request is allowed to remain open|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`
|tlsHandshakeTimeout|The maximum amount of time to wait for a successful TLS handshake|[`time.Duration`](https://pkg.go.dev/time#Duration)|`10s`

## webhooks.auth

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|password|Password|`string`|`<nil>`
|username|Username|`string`|`<nil>`

## webhooks.proxy

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|url|Optional HTTP proxy to use when invoking WebHooks|`string`|`<nil>`

## webhooks.retry

|Key|Description|Type|Default Value|
|---|-----------|----|-------------|
|count|The maximum number of times to retry|`int`|`5`
|enabled|Enables retries|`boolean`|`false`
|initWaitTime|The initial retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`250ms`
|maxWaitTime|The maximum retry delay|[`time.Duration`](https://pkg.go.dev/time#Duration)|`30s`