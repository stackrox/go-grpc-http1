module golang.stackrox.io/grpc-http1/_integration-tests

go 1.16

require (
	github.com/stretchr/testify v1.7.2
	golang.org/x/net v0.0.0-20201021035429-f5854403a974
	golang.stackrox.io/grpc-http1 v0.0.0+incompatible
	google.golang.org/grpc v1.47.0
	google.golang.org/grpc/examples v0.0.0-20200825170228-39ef2aaf62df
)

replace golang.stackrox.io/grpc-http1 => ../
