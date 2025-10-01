GOCMD ?= go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean

INSTALL := /usr/bin/install
PREFIX ?= /usr

BUNDLE := bundle.so

all: build

build:
	$(GOBUILD) -buildmode=plugin -o $(BUNDLE) ./

clean:
	$(GOCLEAN)
	rm -f $(BUNDLE)

install:
	$(INSTALL) -D -m 755 $(BUNDLE) $(PREFIX)/libexec/opensvc/bundle.so

uninstall:
	rm -f $(PREFIX)/libexec/opensvc/bundle.so
