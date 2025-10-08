module golang.stackrox.io/grpc-http1/_integration-tests

go 1.24.0

require (
	github.com/stretchr/testify v1.11.1
	golang.org/x/net v0.44.0
	golang.stackrox.io/grpc-http1 v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.76.0
	google.golang.org/grpc/examples v0.0.0-20250128160859-73e447014dfa
)

require (
	github.com/coder/websocket v1.8.14 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/glog v1.2.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250804133106-a7a43d27e69b // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace golang.stackrox.io/grpc-http1 => ../
