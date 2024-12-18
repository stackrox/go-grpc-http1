module golang.stackrox.io/grpc-http1/_integration-tests

go 1.21.0
toolchain go1.22.9

require (
	github.com/stretchr/testify v1.10.0
	golang.org/x/net v0.32.0
	golang.stackrox.io/grpc-http1 v0.0.0+incompatible
	google.golang.org/grpc v1.69.0
	google.golang.org/grpc/examples v0.0.0-20230602173802-c9d3ea567325
)

require (
	github.com/coder/websocket v1.8.12 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/glog v1.2.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241015192408-796eee8c2d53 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace golang.stackrox.io/grpc-http1 => ../
