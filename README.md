grpc-http1: A gRPC via HTTP/1 Enabling Library for Go
====================================================

This library enables using all the functionality of a gRPC server even if it is exposed behind
a reverse proxy which does not support HTTP/2, or only supports it for clients (such as Amazon's ALB).
This is accomplished via either adaptive downgrading to the gRPC-Web response format or utilizing WebSockets.

For a high-level overview, see [this Medium post](https://medium.com/stackrox-engineering/how-to-expose-grpc-services-behind-almost-any-load-balancer-e9ebf8e6d12a).

**Stay tuned for a high-level overview article to the WebSocket solution.**

Connection Compatibility Overview
---------------------------------

The following table shows what can be expected when a client/server instrumented with the capability
offered by this library compared to an unmodified gRPC client/server, both when accessing it directly and
when accessing it via a reverse proxy not supporting HTTP/2.

<table>
<tr><th></th><th colspan="2">Plain Old gRPC Server</th><th colspan="2">HTTP/1 Downgrading gRPC Server</th></tr>
<tr><th></th><th>direct</th><th>behind reverse proxy</th><th>direct</th><th>behind reverse proxy</th></tr>
<tr><td>Plain Old gRPC Client</td><td>:white_check_mark:</td><td>:x:</td><td>:white_check_mark:</td><td>:x:</td></tr>
<tr><td>gRPC-Web downgrade client mode</td><td>:white_check_mark:</td><td>:x:</td><td>:white_check_mark:</td><td>(:white_check_mark:)</td></tr>
<tr><td>gRPC-WebSocket client mode</td><td>:x:</td><td>:x:</td><td>:white_check_mark:</td><td>:white_check_mark:</td></tr>
</table>

The (:white_check_mark:) for the gRPC-Web downgrading client indicates a subset of gRPC calls will be possible, but not
all. These include all calls that do not rely on client-side streaming (i.e., all unary and server-streaming calls).

As you can see, when using the client in gRPC-Web downgrade mode, it is possible to instrument the client **or** the server without any (functional) regressions - there
may be a small but fairly negligible performance penalty. This means rolling this feature out to your clients and
servers does not need to happen in a strictly synchronous fashion. However, you will only be able to work with a server
behind an HTTP/2-incompatible reverse proxy if both the client **and** the server have been instrumented via
this library. To use the client in gRPC-WebSocket mode, both the client **and** server must be instrumented via this library.


Usage
-------------

This library has the canonical import path `golang.stackrox.io/grpc-http1`. It fully supports Go modules
and requires Go version 1.13+ to be built and used. To add it as a dependency in your current project,
run `go get golang.stackrox.io/grpc-http1`.


### Server-side

For using this library on the server-side, you'll need to bypass the regular `(*grpc.Server).Serve` method
and instead use the `ServeHTTP` method of the `*grpc.Server` object -- it is experimental, but we found it
to be fairly stable and reliable.

The only exported function in the `golang.grpc.io/grpc-http1/server` package is `CreateDowngradingHandler`,
which returns a `http.Handler` that can be served by a Go HTTP server. It is crucial this server is
configured to support HTTP/2; otherwise, your clients using the vanilla gRPC client will no longer be able
to talk to it. You can find an example of how to do so in the `_integration-tests/` directory.

### Client-Side

For connecting to a gRPC server via a client-side proxy, use the `ConnectViaProxy` function exported from the
`golang.grpc.io/grpc-http1/client` package. This function has the following signature:
```go
func ConnectViaProxy(ctx context.Context, endpoint string, tlsClientConf *tls.Config, opts ...ConnectOption) (*grpc.ClientConn, error)
```
The first two arguments are the same as for `grpc.DialContext`. The third argument specifies the TLS client
config to be used for connecting to the target address. Note that this is different from the usual gRPC API,
which specifies client TLS config via the `grpc.WithTransportCredentials`. For a plaintext (unencrypted)
connection to the server, pass a `nil` TLS config; however, this does *not* free you from passing the
`grpc.WithInsecure()` gRPC dial option.

The last (variadic) parameter specifies options that modify the dialing behavior. You can pass any gRPC dial
options via `client.DialOpts(...)`; however, the `grpc.WithTransportCredentials` option will not be needed.
By default, adaptive gRPC-Web downgrading is used. To use WebSockets, pass `true` to the `client.UseWebSocket` option.

Another important option is `client.ForceHTTP2()`, which needs to be used for
a plaintext connection to a server that is *not* HTTP/1.1 capable (e.g., the vanilla gRPC server).
This option is ignored when WebSockets are used. Again, check out the
code in the `_integration-tests` directory.
