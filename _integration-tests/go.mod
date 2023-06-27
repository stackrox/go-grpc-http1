module golang.stackrox.io/grpc-http1/_integration-tests

go 1.18

require (
	github.com/stretchr/testify v1.8.4
	golang.org/x/net v0.11.0
	golang.stackrox.io/grpc-http1 v0.0.0+incompatible
	google.golang.org/grpc v1.56.1
	google.golang.org/grpc/examples v0.0.0-20230602173802-c9d3ea567325
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/glog v1.1.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/klauspost/compress v1.10.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/text v0.10.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230525234030-28d5490b6b19 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	nhooyr.io/websocket v1.8.7 // indirect
)

replace golang.stackrox.io/grpc-http1 => ../
