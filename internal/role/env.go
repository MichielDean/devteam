package role

import "os"

// osGetenv is a tiny indirection so tests can stub env reads without polluting
// the package namespace with a mock. Kept here so the resolver stays pure
// except for the one env read.
var osGetenv = os.Getenv