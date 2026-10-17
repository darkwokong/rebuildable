package version

// Version is replaced at release build time with -ldflags. Keeping a useful
// development default makes `go run` and local tests self-describing.
var Version = "dev"
