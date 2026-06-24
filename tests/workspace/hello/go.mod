module example.com/hello

go 1.21

require (
	example.com/stringutil v0.0.0
	golang.org/x/text v0.21.0
)

replace example.com/stringutil => ../stringutil
