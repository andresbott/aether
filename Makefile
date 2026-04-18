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
			if [ $$(echo "$$coverage < $(COVERAGE_THRESHOLD)" | bc -l) -eq 1 ]; then \
				echo "❌ Coverage in $$pkg is below $(COVERAGE_THRESHOLD)!"; \
				fail=1; \
			fi; \
			rm -f coverage.out; \
		else \
			echo "⚠️ No coverage data for $$pkg"; \
			fail=1; \
		fi; \
	done; \
	exit $$fail

benchmark: ## run go benchmarks
	@go test -run=^$$ -bench=. ./...

license-check: ## check for invalid licenses
	# depends on : https://github.com/elastic/go-licence-detector
	@go list -m -mod=readonly -json all | go-licence-detector -includeIndirect -rules allowedLicenses.json -overrides overrideLicenses.json

.PHONY: verify
verify: test ui-test license-check lint benchmark coverage ## run all tests

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
