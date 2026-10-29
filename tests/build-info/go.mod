module example.com/build-info

go 1.24

require (
	example.com/lib v0.0.0
	github.com/samber/lo v1.47.0
	github.com/spf13/cobra v1.10.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/text v0.16.0 // indirect
)

replace example.com/lib => ./lib
