OSVC_CONTEXT =

GOCMD ?= go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGEN = $(GOCMD) generate
GOVET = $(GOCMD) vet

OM = ./bin/om
OX = ./bin/ox
COMPOBJ = ./bin/compobj

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
