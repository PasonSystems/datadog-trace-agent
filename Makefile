# This Makefile is used within the release process of the main Datadog Agent to pre-package datadog-trace-agent:
# https://github.com/DataDog/datadog-agent/blob/2b7055c/omnibus/config/software/datadog-trace-agent.rb

# if the TRACE_AGENT_VERSION environment variable isn't set, default to 0.99.0
TRACE_AGENT_VERSION := $(if $(TRACE_AGENT_VERSION),$(TRACE_AGENT_VERSION), 0.99.0)

# break up the version
SPLAT = $(subst ., ,$(TRACE_AGENT_VERSION))
VERSION_MAJOR = $(shell echo $(word 1, $(SPLAT)) | sed 's/[^0-9]*//g')
VERSION_MINOR = $(shell echo $(word 2, $(SPLAT)) | sed 's/[^0-9]*//g')
VERSION_PATCH = $(shell echo $(word 3, $(SPLAT)) | sed 's/[^0-9]*//g')

# account for some defaults
VERSION_MAJOR := $(if $(VERSION_MAJOR),$(VERSION_MAJOR), 0)
VERSION_MINOR := $(if $(VERSION_MINOR),$(VERSION_MINOR), 0)
VERSION_PATCH := $(if $(VERSION_PATCH),$(VERSION_PATCH), 0)

ifeq ($(OS),Windows_NT)
    # when calling CD, gnu make (on windows) seems to use the cygwin implementation,
	# and needs GOPATH with `/` as path separator rather than `\`.  For windows,
	# the omnibus def will supply us both.  Use the `/` path for DEPDIR, which is
	# the input to CD below.
    DEPDIR = $(GOPATHNIX)/src/github.com/golang/dep
else
    DEPDIR = $(GOPATH)/src/github.com/golang/dep
endif
deps:
	go get -d github.com/golang/dep/cmd/dep
	cd $(DEPDIR) && git reset --hard v0.3.2 && cd -
	go install github.com/golang/dep/cmd/dep
	dep ensure

install: deps
	# prepares all dependencies by running the 'deps' task, generating
	# versioning information and installing the binary.
	go generate ./info
	go install ./cmd/trace-agent

ci: deps
	# task used by CI
	go get -u github.com/golang/lint/golint/...
	golint ./cmd/trace-agent ./filters ./fixtures ./info ./quantile ./quantizer ./sampler ./statsd ./watchdog ./writer
	go test ./...

windows:
	# pre-packages resources needed for the windows release
	windmc --target pe-x86-64 -r cmd/trace-agent/windows_resources cmd/trace-agent/windows_resources/trace-agent-msg.mc
	windres  --define MAJ_VER=$(VERSION_MAJOR) --define MIN_VER=$(VERSION_MINOR) --define PATCH_VER=$(VERSION_PATCH) -i cmd/trace-agent/windows_resources/trace-agent.rc --target=pe-x86-64 -O coff -o cmd/trace-agent/rsrc.syso
