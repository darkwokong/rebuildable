module github.com/jayeshsarvaiya/rebuildable

go 1.27

toolchain go1.27.2

require github.com/spf13/cobra v1.10.2

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

// The Cobra documentation and Windows-only helpers are not part of the
// Linux binary. These local compatibility modules keep the build reproducible
// when the default test command is run without network access.
replace github.com/cpuguy83/go-md2man/v2 => ./third_party/go-md2man

replace github.com/inconshreveable/mousetrap => ./third_party/mousetrap

replace github.com/spf13/pflag => github.com/spf13/pflag v1.0.10

replace go.yaml.in/yaml/v3 => ./third_party/yaml
