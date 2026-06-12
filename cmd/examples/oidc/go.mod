// This example depends on the nullspace-oidc contrib module, so it lives in
// its own Go module to keep the main nullspace module free of that
// dependency. Building it requires a sibling checkout of nullspace-oidc
// (see the replace directives below).
module github.com/frob/nullspace/cmd/examples/oidc

go 1.25.6

require (
	github.com/frob/nullspace v0.0.0
	github.com/frob/nullspace-oidc v0.0.0
)

require (
	github.com/coreos/go-oidc/v3 v3.14.1 // indirect
	github.com/go-jose/go-jose/v4 v4.0.5 // indirect
	github.com/pelletier/go-toml/v2 v2.3.0 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/frob/nullspace => ../../..

replace github.com/frob/nullspace-oidc => ../../../../nullspace-oidc
