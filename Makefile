all:
	go build
	$(MAKE) -C tests
