OSVC_CONTEXT =

GOCMD ?= go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGEN = $(GOCMD) generate
GOVET = $(GOCMD) vet

MKDIR = /usr/bin/mkdir
INSTALL = /usr/bin/install
PREFIX = /usr

OM = bin/om
OX = bin/ox
COMPOBJ = bin/compobj
COMPOBJ_D = share/opensvc/compliance

.PHONY: strip dist

all: vet test build

build: api om ox compobj

api:
	$(GOGEN) ./daemon/api

clean:
	$(GOCLEAN)
	rm -f $(OM) $(OX)

om:
	$(GOBUILD) -o $(OM) -v ./cmd/om/

ox:
	$(GOBUILD) -o $(OX) -v ./cmd/ox/

compobj:
	$(GOBUILD) -o $(COMPOBJ) -v ./util/compobj/

test:
	$(GOTEST) ./...

vet:
	$(GOVET) ./...

install:
	$(MKDIR) -p $(PREFIX)/bin
	$(MKDIR) -p $(PREFIX)/$(COMPOBJ_D)
	$(INSTALL) -m 755 $(OM) $(PREFIX)/$(OM)
	$(INSTALL) -m 755 $(OX) $(PREFIX)/$(OX)
	$(INSTALL) -m 755 $(COMPOBJ) $(PREFIX)/$(COMPOBJ)
	$(PREFIX)/$(COMPOBJ) -i $(PREFIX)/$(COMPOBJ_D)

strip:
	strip --strip-all $(PREFIX)/$(OM) $(PREFIX)/$(OX) $(PREFIX)/$(COMPOBJ)

dist:
	mkdir -p dist
	tar czvf dist/om.tar.gz $(PREFIX)/$(OM) $(PREFIX)/$(OX) $(PREFIX)/$(COMPOBJ) $(PREFIX)/$(COMPOBJ_D)


