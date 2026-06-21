package main

const (
	version           = "0.2.7"
	maxChainLength    = 10
	headerBytes       = 256
	encHeadBytes      = 160
	encDataBytes      = 64
	headersBytes      = headerBytes * maxChainLength
	bodyBytes         = 17920
	messageBytes      = headersBytes + bodyBytes
	base64LineWrap    = 64
	connectionTimeout = 60
	chunkSize         = 1024 * 2
	maxMessageSize    = 8 * 1024
	proxyAddress      = "127.0.0.1:1080"
	configFileName    = "Mixfit.json"
	maxRetries        = 3
	retryDelay        = 1
)
