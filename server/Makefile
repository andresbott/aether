default: help

#==========================================================================================
##@ Testing
#==========================================================================================
test: ## run fast go tests
	@go test ./... -cover

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
verify: ## run all server checks; runs every check and fails if any fail
	@fail=0; \
	for target in test license-check lint benchmark coverage; do \
		echo "==================== make $$target ===================="; \
		$(MAKE) --no-print-directory $$target || fail=1; \
	done; \
	if [ $$fail -ne 0 ]; then \
		echo "❌ server verify failed (see above)"; \
		exit 1; \
	fi; \
	echo "✅ server verify passed"

coverage-report: ## generate a coverage report
	go test -covermode=count -coverpkg=./... -coverprofile coverage.cover.out  ./...
	@go tool cover -func=coverage.cover.out | tee coverage_internal.report
	go tool cover -html coverage.cover.out -o cover.html
	open cover.html

#==========================================================================================
##@ Running
#==========================================================================================
run: ## start the GO service with the dev config (zarf/localdata/config.yaml)
	@AETHER_ENV_LOGLEVEL="debug" go run main.go start -c zarf/localdata/config.yaml

LAB_ADDR ?= :8099
.PHONY: run-lab
run-lab: ## start the covergen cover-art tuning lab (dev-only; http://localhost:8099 — override with LAB_ADDR=:9000)
	@go run ./libs/covergen/lab -addr "$(LAB_ADDR)"

proxy: ## smoke-test proxy for auth proxy-header mode: make proxy USER=admin GROUP=aether-admin (GROUP optional)
	@# USER is also a shell env var (the login name), so require it explicitly
	@# on the command line — inheriting it would silently proxy as $$USER.
	@[ "$(origin USER)" = "command line" ] || ( echo ">> USER is not set, usage: make proxy USER=admin GROUP=aether-admin"; exit 1 )
	@go run ./zarf/devproxy -user "$(USER)" -groups "$(GROUP)"

DATA_DIR ?= ./zarf/localdata/data
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
#  Help
#==========================================================================================
.PHONY: help
help: # Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
