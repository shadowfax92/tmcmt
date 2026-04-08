PREFIX ?= $(HOME)/bin
VERSION ?= 0.1.0

build:
	go build -ldflags "-X tmcmt/cmd.Version=$(VERSION)" -o tmcmt .

install: build
	cp tmcmt $(PREFIX)/tmcmt
	codesign --force --sign - $(PREFIX)/tmcmt

clean:
	rm -f tmcmt

.PHONY: build install clean
