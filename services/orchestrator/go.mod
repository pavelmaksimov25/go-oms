module github.com/pavelmaksimov25/go-oms/services/orchestrator

go 1.25.3

require (
	github.com/pavelmaksimov25/go-oms/pkg v0.0.0
	github.com/segmentio/kafka-go v0.4.48
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
)

replace github.com/pavelmaksimov25/go-oms/pkg => ../../pkg
