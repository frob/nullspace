// This example depends on the nullspace-grpc contrib module, so it lives in
// its own Go module to keep the main nullspace module free of that
// dependency. Building it requires a sibling checkout of nullspace-grpc
// (see the replace directives below).
module github.com/frob/nullspace/cmd/examples/grpc

go 1.25.6

require (
	github.com/frob/nullspace v0.0.0
	github.com/frob/nullspace-grpc v0.0.0
)

require (
	github.com/pelletier/go-toml/v2 v2.3.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/grpc v1.72.2 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/frob/nullspace => ../../..

replace github.com/frob/nullspace-grpc => ../../../../nullspace-grpc
