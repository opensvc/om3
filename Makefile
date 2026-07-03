OSVC_CONTEXT =

GOCMD ?= go
GOBUILD := $(GOCMD) build
GOBUILDRACE := GORACE="halt_on_error=1" $(GOCMD) build -race
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGEN := $(GOCMD) generate
GOVET := $(GOCMD) vet
GOINSTALL := $(GOCMD) install
SSHKEY ?= /root/.ssh/opensvc
SCP := scp -i $(SSHKEY)
SSH := ssh -i $(SSHKEY)

STRIP := /usr/bin/strip
MKDIR := /usr/bin/mkdir
INSTALL := /usr/bin/install
PREFIX ?= /usr

DIST := dist
OM := bin/om
OX := bin/ox
COMPOBJ := bin/compobj
COMPOBJ_D := share/opensvc/compliance
LOCAL_HOSTNAME := $(shell hostname 2>/dev/null || echo $${HOSTNAME:-localhost})

.PHONY: version dist deploy restart

all: clean vet test race build dist

all-race: clean vet test race build-race dist

build: version api om ox compobj

build-race: version api om-race ox-race compobj-race

deps:
	$(GOINSTALL) github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

api:
	$(GOGEN) ./daemon/api

clean:
	$(GOCLEAN)
	$(GOCLEAN) -testcache
	rm -f $(OM) $(OX)

compobj:
	$(GOBUILD) -o $(COMPOBJ) ./util/compobj/

compobj-race:
	$(GOBUILDRACE) -o $(COMPOBJ) ./util/compobj/

deploy:
	@for node in $(shell $(OM) node ls); do \
		echo "Deploying $(OM) to $$node..."; \
		TEMP_OM_FILE="/tmp/om-$(shell head /dev/urandom | tr -dc A-Za-z0-9 | head -c 10)"; \
		TEMP_OX_FILE="/tmp/ox-$(shell head /dev/urandom | tr -dc A-Za-z0-9 | head -c 10)"; \
		if [ "$$node" = "$(LOCAL_HOSTNAME)" ]; then \
			$(INSTALL) -m 755 $(OM) $(PREFIX)/$(OM); \
			$(INSTALL) -m 755 $(OX) $(PREFIX)/$(OX); \
			$(PREFIX)/$(OM) daemon restart; \
		else \
			$(SCP) "$(OM)" "$$node:$$TEMP_OM_FILE" && \
			$(SCP) "$(OX)" "$$node:$$TEMP_OX_FILE" && \
			$(SSH) "$$node" \
					"sudo install -m 755 $$TEMP_OM_FILE $(PREFIX)/$(OM) \
					&& sudo install -m 755 $$TEMP_OX_FILE $(PREFIX)/$(OX) \
					&& rm $$TEMP_OM_FILE $$TEMP_OX_FILE \
					&& $(PREFIX)/$(OM) daemon restart" || { \
				echo "Deployment failed for $$node. Aborting."; \
				exit 1; \
			}; \
		fi; \
	done
	@echo "Deployment to all nodes completed successfully."

dist:
	$(MKDIR) -p $(DIST)/bin
	$(MKDIR) -p $(DIST)/$(COMPOBJ_D)
	$(INSTALL) -m 755 $(OM) $(DIST)/$(OM)
	$(INSTALL) -m 755 $(OX) $(DIST)/$(OX)
	$(INSTALL) -m 755 $(COMPOBJ) $(DIST)/$(COMPOBJ)
	$(DIST)/$(COMPOBJ) -r -i $(DIST)/$(COMPOBJ_D)
	$(STRIP) --strip-all $(DIST)/$(OM) $(DIST)/$(OX) $(DIST)/$(COMPOBJ)
	VERSION=`git describe --tags --abbrev` && cd $(DIST) && tar czvf opensvc-$$VERSION.tar.gz $(OM) $(OX) $(COMPOBJ) $(COMPOBJ_D) && cd -

install:
	$(MKDIR) -p $(PREFIX)/bin
	$(MKDIR) -p $(PREFIX)/$(COMPOBJ_D)
	$(INSTALL) -m 755 $(OM) $(PREFIX)/$(OM)
	$(INSTALL) -m 755 $(OX) $(PREFIX)/$(OX)
	$(INSTALL) -m 755 $(COMPOBJ) $(PREFIX)/$(COMPOBJ)
	$(PREFIX)/$(COMPOBJ) -i $(PREFIX)/$(COMPOBJ_D)

om:
	$(GOBUILD) -o $(OM) ./cmd/om/

om-race:
	$(GOBUILDRACE) -o $(OM) ./cmd/om/

ox:
	$(GOBUILD) -o $(OX) ./cmd/ox/

ox-race:
	$(GOBUILDRACE) -o $(OX) ./cmd/ox/

test-race:
	$(GOTEST) -p 1 -timeout 240s ./... -race

restart:
	$(PREFIX)/$(OM) daemon restart

test:
	$(GOTEST) -p 1 -timeout 60s ./...

test-cover:
	$(GOTEST) -p 1 -timeout 60s -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

testinfo:
	TEST_LOG_LEVEL=info $(GOTEST) -p 1 -timeout 60s ./...

version:
	git describe --tags --abbrev >util/version/text/VERSION

vet:
	$(GOVET) ./...


help:
	@echo "Available targets:"
	@echo "  api            - Generate the api code from api.yaml"
	@echo "  build          - Build om and ox"
	@echo "  compobj        - Build the compliance modules pack"
	@echo "  dist           - Build, strip binaries and make a tarball"
	@echo "  om             - Build om"
	@echo "  ox             - Build ox"
	@echo "  install        - Install o[mx] to /usr/bin"
	@echo "  restart        - Restart the daemon"
	@echo "  deploy         - Install and restart on all nodes"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Update dependencies"
	@echo "  test           - Run tests"
	@echo "  test-cover     - Run tests with coverage"
	@echo "  vet            - Run go static analyzer"
	@echo "  version        - Update the version string from git status"
	@echo
	@echo "Available -race targets:"
	@echo "  build-race     - Build om and ox if race-free"
	@echo "  compobj-race   - Build the compliance modules pack if race-free"
	@echo "  om-race        - Build om if race-free"
	@echo "  ox-race        - Build ox if race-free"
	@echo "  test-race      - Run tests with race detection"

