module golang.stackrox.io/grpc-http1/_integration-tests

go 1.14

require (
	github.com/stretchr/testify v1.5.1
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	golang.stackrox.io/grpc-http1 v0.0.0+incompatible
	google.golang.org/grpc v1.31.1
	google.golang.org/grpc/examples v0.0.0-20200825170228-39ef2aaf62df
)

replace golang.stackrox.io/grpc-http1 => ../
