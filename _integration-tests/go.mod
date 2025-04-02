module golang.stackrox.io/grpc-http1/_integration-tests

go 1.22.5
toolchain go1.24.1

require (
	github.com/stretchr/testify v1.10.0
	golang.org/x/net v0.38.0
	golang.stackrox.io/grpc-http1 v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.71.1
	google.golang.org/grpc/examples v0.0.0-20250128160859-73e447014dfa
)

require (
	github.com/coder/websocket v1.8.13 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/glog v1.2.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250127172529-29210b9bc287 // indirect
	google.golang.org/protobuf v1.36.4 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace golang.stackrox.io/grpc-http1 => ../
