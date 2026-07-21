BINARY:=wakeci
PWD:=$(shell pwd)
VERSION=0.0.0
VUE_VERSION_SUFFIX:=$(shell date +"%d%b")
MONOVA:=$(shell which monova dot 2> /dev/null)

export PATH := $(PATH):$(shell go env GOPATH)/bin

version:
ifdef MONOVA
override VERSION=$(shell monova)
else
	$(info "Install monova (https://github.com/jsnjack/monova) to calculate version")
endif

export VITE_VERSION = ${VERSION}-${VUE_VERSION_SUFFIX}

.ONESHELL:
src/backend/wakeci: version src/backend/*.go
	cd src/backend
	rm -rf assets
	cp -r ../frontend/dist/ assets
	mkdir -p docs
	touch docs/swagger.json
	which swag 2> /dev/null || grm install swaggo/swag
	swag init --parseDependency --parseInternal --parseDepth 1 || exit 99
	CGO_ENABLED=0 go build -ldflags="-X main.Version=${VERSION}" -o ${BINARY}

.ONESHELL:
bin/wakeci: src/backend/wakeci
	cd src/backend
	cp wakeci ${PWD}/bin/

runf:
	cd src/frontend && npm run serve

runb: src/backend/wakeci
	cd src/backend
	ls *.go | entr -sr "cd ../../ && make src/backend/wakeci && ./src/backend/wakeci"

test_go:
	cd src/backend && go test ./...

test: test_go

E2E_PORT := 18081

.ONESHELL:
testprod: test_go build
	set -e
	TMPDIR=$$(mktemp -d)
	mkdir -p $$TMPDIR/workdir $$TMPDIR/jobdir
	echo "port: \"$(E2E_PORT)\"" > $$TMPDIR/Wakefile.yaml
	echo "workdir: $$TMPDIR/workdir" >> $$TMPDIR/Wakefile.yaml
	echo "jobdir: $$TMPDIR/jobdir" >> $$TMPDIR/Wakefile.yaml
	echo "secretsfile: $(PWD)/secrets.yaml" >> $$TMPDIR/Wakefile.yaml
	cleanup() {
		code=$$?
		kill $$WAKECI_PID 2>/dev/null
		if [ $$code -ne 0 ]; then
			echo "==> e2e run failed - backend trace log (/tmp/wakeci.log):"
			tail -n 150 /tmp/wakeci.log 2>/dev/null
		fi
		rm -rf $$TMPDIR
	}
	./bin/wakeci --config $$TMPDIR/Wakefile.yaml --trace &
	WAKECI_PID=$$!
	trap cleanup EXIT
	for i in $$(seq 1 40); do curl -sf http://localhost:$(E2E_PORT)/ >/dev/null 2>&1 && break; sleep 0.25; done
	cd src/frontend && WAKECI_E2E_PORT=$(E2E_PORT) npm run test:prod

# testdev assumes `make runf` and `make runb` are already running in separate
# terminals; it is a manual/interactive dev-loop tool, not isolated e2e.
testdev: test_go
	cd src/frontend && npm run test:dev

vet:
	cd src/backend && go vet ./...

fmt:
	@command -v goimports >/dev/null 2>&1 || { \
	  echo "goimports is not installed. Install it with:"; \
	  echo "  go install golang.org/x/tools/cmd/goimports@latest"; \
	  exit 1; \
	}
	cd src/backend && goimports -w .

lint: vet
	@command -v golangci-lint >/dev/null 2>&1 || { \
	  echo "golangci-lint is not installed. Install it with:"; \
	  echo "  grm install golangci/golangci-lint"; \
	  exit 1; \
	}
	cd src/backend && golangci-lint run

check: fmt vet build test lint
	@echo "==> make check: all green"

standards:
	curl -sL https://raw.githubusercontent.com/jsnjack/standards/master/AGENTS.universal.md \
	    -o AGENTS.universal.md
	curl -sL https://raw.githubusercontent.com/jsnjack/standards/master/AGENTS.go.md \
	    -o AGENTS.go.md

buildf:
	cd src/frontend && npm run build

build: buildf bin/wakeci

release: build
	grm release jsnjack/wakeci -f bin/${BINARY} -t "v`monova`"

.ONESHELL:
clean:
	rm -rf workdir/*
	rm -rf src/frontend/dist

clean_jobs:
	cd workdir && find . -name "*.yaml" -delete

.ONESHELL:
viewdb:
	cd workdir
	rm -f view.db
	cp wakeci.db view.db
	bolter -f view.db

.PHONY: runb runf version clean clean_jobs testprod testdev test_go test vet fmt lint check standards buildf build release
