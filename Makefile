OSVC_CONTEXT =

GOCMD ?= go
GOBUILD := $(GOCMD) build
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
LOCAL_HOSTNAME := $(shell hostname)

.PHONY: version dist deploy restart

all: clean vet test race build dist

build: version api om ox compobj

deps:
	$(GOINSTALL) github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

api:
	$(GOGEN) ./daemon/api

clean:
	$(GOCLEAN)
	$(GOCLEAN) -testcache
	rm -f $(OM) $(OX)

om:
	$(GOBUILD) -o $(OM) ./cmd/om/

ox:
	$(GOBUILD) -o $(OX) ./cmd/ox/

compobj:
	$(GOBUILD) -o $(COMPOBJ) ./util/compobj/

test:
	$(GOTEST) -p 1 -timeout 60s ./...

testinfo:
	TEST_LOG_LEVEL=info $(GOTEST) -p 1 -timeout 60s ./...

race:
	$(GOTEST) -p 1 -timeout 240s ./... -race

vet:
	$(GOVET) ./...

install:
	$(MKDIR) -p $(PREFIX)/bin
	$(MKDIR) -p $(PREFIX)/$(COMPOBJ_D)
	$(INSTALL) -m 755 $(OM) $(PREFIX)/$(OM)
	$(INSTALL) -m 755 $(OX) $(PREFIX)/$(OX)
	$(INSTALL) -m 755 $(COMPOBJ) $(PREFIX)/$(COMPOBJ)
	$(PREFIX)/$(COMPOBJ) -i $(PREFIX)/$(COMPOBJ_D)

version:
	git describe --tags --abbrev >util/version/text/VERSION

dist:
	$(MKDIR) -p $(DIST)/bin
	$(MKDIR) -p $(DIST)/$(COMPOBJ_D)
	$(INSTALL) -m 755 $(OM) $(DIST)/$(OM)
	$(INSTALL) -m 755 $(OX) $(DIST)/$(OX)
	$(INSTALL) -m 755 $(COMPOBJ) $(DIST)/$(COMPOBJ)
	$(DIST)/$(COMPOBJ) -r -i $(DIST)/$(COMPOBJ_D)
	$(STRIP) --strip-all $(DIST)/$(OM) $(DIST)/$(OX) $(DIST)/$(COMPOBJ)
	VERSION=`git describe --tags --abbrev` && cd $(DIST) && tar czvf opensvc-$$VERSION.tar.gz $(OM) $(OX) $(COMPOBJ) $(COMPOBJ_D) && cd -

restart:
	$(PREFIX)/$(OM) daemon restart

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

