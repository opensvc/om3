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
COMPOBJ_D = $(PREFIX)/share/opensvc/compliance

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
	$(MKDIR) -p $(COMPOBJ_D)
	$(INSTALL) -m 755 $(OM) $(PREFIX)/$(OM)
	$(INSTALL) -m 755 $(OX) $(PREFIX)/$(OX)
	$(INSTALL) -m 755 $(COMPOBJ) $(PREFIX)/$(COMPOBJ)
	$(PREFIX)/$(COMPOBJ) -i $(COMPOBJ_D)
