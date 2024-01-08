OSVC_CONTEXT =

GOCMD ?= go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGEN = $(GOCMD) generate
GOVET = $(GOCMD) vet

OM = ./bin/om
OX = ./bin/ox

all: test om ox

api:
	$(GOGEN) ./daemon/api

clean:
	$(GOCLEAN)
	rm -f $(OM) $(OX)

om: api
	$(GOBUILD) -o $(OM) -v ./cmd/om/

ox: api
	$(GOBUILD) -o $(OX) -v ./cmd/ox/

test:
	$(GOTEST) ./...

vet:
	$(GOVET) ./...
