module example.com/main

go 1.13

require (
	example.com/a v0.0.0
	example.com/a/nested v0.0.0
	example.com/b v1.0.0
)

replace example.com/a => ../deps/a

replace example.com/a/nested => ../deps/a/nested

replace example.com/b v1.0.0 => ../deps/b

replace example.com/c => ../deps/c
