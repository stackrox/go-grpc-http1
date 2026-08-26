module golang.stackrox.io/grpc-http1/_integration-tests

go 1.25.0

require (
	github.com/stretchr/testify v1.12.1
	golang.org/x/net v0.58.0
	golang.stackrox.io/grpc-http1 v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.83.1
	google.golang.org/grpc/examples v0.0.0-20250128160859-73e447014dfa
)

require (
	github.com/coder/websocket v1.8.15 // indirect
	github.com/golang/glog v1.2.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace golang.stackrox.io/grpc-http1 => ../
