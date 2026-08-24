COMMIT_SHA_SHORT ?= $(shell git rev-parse --short=12 HEAD)
PWD_DIR := ${CURDIR}

default: help

#==========================================================================================
##@ Testing
#==========================================================================================
test: ## run fast go tests
	@go test ./... -cover

ui-test: ## run webui unit tests
	@cd webui && npm test

spec-lint: ## lint docs/openapi/aether-v1.yaml against .spectral.yaml (header-safe bounded-URL invariant)
	@cd webui && npm run spec-lint

lint: ## run go linter
	# depends on https://github.com/golangci/golangci-lint
	@golangci-lint run

COVERAGE_THRESHOLD ?= 70
.PHONY: coverage
coverage:
	@fail=0; \
	for pkg in $$(go list ./internal/...); do \
		go test -coverprofile=coverage.out -covermode=atomic $$pkg > /dev/null; \
		if [ -f coverage.out ]; then \
			coverage=$$(go tool cover -func=coverage.out | grep total: | awk '{print $$3}' | sed 's/%//'); \
			if [ -z "$$coverage" ]; then \
				echo "⚠️  $$pkg: no coverage total"; \
				fail=1; \
			elif awk "BEGIN { exit !($$coverage < $(COVERAGE_THRESHOLD)) }"; then \
				echo "❌ $$pkg: $$coverage% (below $(COVERAGE_THRESHOLD)%)"; \
				fail=1; \
			else \
				echo "✅ $$pkg: $$coverage%"; \
			fi; \
			rm -f coverage.out; \
		else \
			echo "⚠️  $$pkg: no coverage data"; \
			fail=1; \
		fi; \
	done; \
	if [ $$fail -ne 0 ]; then \
		echo "❌ coverage below threshold ($(COVERAGE_THRESHOLD)%)"; \
		exit 1; \
	fi; \
	echo "✅ coverage: all packages >= $(COVERAGE_THRESHOLD)%"

benchmark: ## run go benchmarks
	@go test -run=^$$ -bench=. ./...

license-check: ## check for invalid licenses
	# depends on : https://github.com/elastic/go-licence-detector
	@go list -m -mod=readonly -json all | go-licence-detector -includeIndirect -rules allowedLicenses.json -overrides overrideLicenses.json

.PHONY: verify
verify: ## run all checks; runs every check and fails if any fail
	@fail=0; \
	for target in test ui-test spec-lint license-check lint benchmark coverage; do \
		echo "==================== make $$target ===================="; \
		$(MAKE) --no-print-directory $$target || fail=1; \
	done; \
	if [ $$fail -ne 0 ]; then \
		echo "❌ verify failed (see above)"; \
		exit 1; \
	fi; \
	echo "✅ verify passed"

coverage-report: ## generate a coverage report
	go test -covermode=count -coverpkg=./... -coverprofile coverage.cover.out  ./...
	@go tool cover -func=coverage.cover.out | tee coverage_internal.report
	go tool cover -html coverage.cover.out -o cover.html
	open cover.html

#==========================================================================================
##@ Running
#==========================================================================================
run: ## start the GO service (uses built-in defaults; optional -c config.yaml)
	@AETHER_ENV_LOGLEVEL="debug" go run main.go start

run-ui: package-ui run## build the UI and start the GO service

proxy: ## smoke-test proxy for auth proxy-header mode: make proxy USER=admin GROUP=aether-admin (GROUP optional)
	@# USER is also a shell env var (the login name), so require it explicitly
	@# on the command line — inheriting it would silently proxy as $$USER.
	@[ "$(origin USER)" = "command line" ] || ( echo ">> USER is not set, usage: make proxy USER=admin GROUP=aether-admin"; exit 1 )
	@go run ./zarf/devproxy -user "$(USER)" -groups "$(GROUP)"

DATA_DIR ?= ./data
.PHONY: reset-data
reset-data: ## delete the local data dir — user DB, image cache, metadata, task logs, session/PAT keys (FORCE=1 skips the prompt)
	@if [ ! -e "$(DATA_DIR)" ]; then \
		echo "nothing to remove: '$(DATA_DIR)' does not exist"; \
	else \
		if [ "$(FORCE)" != "1" ]; then \
			printf ">> delete ALL local data in '%s' (DB, caches, metadata, keys)? [y/N] " "$(DATA_DIR)"; \
			read ans; \
			case "$$ans" in [yY]|[yY][eE][sS]) ;; *) echo "aborted"; exit 1;; esac; \
		fi; \
		rm -rf "$(DATA_DIR)"; \
		echo "✅ removed '$(DATA_DIR)' (recreated on next 'make run')"; \
	fi

#==========================================================================================
##@ Building
#==========================================================================================
package-ui: build-ui ## build the web and copy into Go package
	rm -rf ./app/spa/files/ui*
	mkdir -p ./app/spa/files/ui
	cp -r ./webui/dist/* ./app/spa/files/ui/
	touch ./app/spa/files/ui/.gitkeep
build-ui:
	@cd webui && \
	npm install && \
	npm run build

build: package-ui ## use goreleaser to build to current OS/Arch
	@goreleaser build --snapshot --clean --single-target

.PHONY: icons
icons: ## re-render the SPA icon set from zarf/icon into webui/public (needs inkscape + imagemagick)
	@./zarf/icon/render.sh

#==========================================================================================
##@ Release
#==========================================================================================

.PHONY: check-branch
check-branch:
	@current_branch=$$(git symbolic-ref --short HEAD) && \
	if [ "$$current_branch" != "main" ]; then \
		echo "Error: You are on branch '$$current_branch'. Please switch to 'main'."; \
		exit 1; \
	fi

.PHONY: check-git-clean
check-git-clean: # check if git repo is clean
	@git diff --quiet

tag: check-git-clean check-branch ## create a git tag to publish a new release
	@[ "${version}" ] || ( echo ">> version is not set, usage: make release version=\"v1.2.3\" "; exit 1 )
	@git tag -d $(version) || true
	@git tag -a $(version) -m "Release version: $(version)"
	@git push --delete origin $(version) || true
	@git push origin $(version) || true

clean: ## clean build env
	@rm -rf dist


#==========================================================================================
#  Help
#==========================================================================================
.PHONY: help
help: # Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
