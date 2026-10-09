COMMIT_SHA_SHORT ?= $(shell git rev-parse --short=12 HEAD)
PWD_DIR := ${CURDIR}

# Server-only targets live in server/Makefile; the ones below delegate to it so
# everything can still be run from the repo root. Recipes spell out $(MAKE)
# literally so make treats them as recursive (-n, -j jobserver).
IN_SERVER := --no-print-directory -C server
# Same for the website: site/Makefile holds its targets (theme-update, new, …).
IN_SITE := --no-print-directory -C site
# And for the desktop player: player/Makefile (prepare, run, lint, test, build, …).
IN_PLAYER := --no-print-directory -C player

default: help

#==========================================================================================
##@ Testing
#==========================================================================================
test: ## run fast go tests
	@$(MAKE) $(IN_SERVER) test

ui-test: ## run webui unit tests
	@cd webui && npm test

spec-lint: ## lint docs/openapi/aether-v0.yaml against .spectral.yaml (header-safe bounded-URL invariant)
	@cd webui && npm run spec-lint

lint: ## run go linter
	@$(MAKE) $(IN_SERVER) lint

.PHONY: coverage
coverage:
	@$(MAKE) $(IN_SERVER) coverage

benchmark: ## run go benchmarks
	@$(MAKE) $(IN_SERVER) benchmark

license-check: ## check for invalid licenses
	@$(MAKE) $(IN_SERVER) license-check

.PHONY: verify
verify: ## run all checks (server verify + webui + player); runs every check and fails if any fail
	@fail=0; \
	echo "==================== make -C server verify ===================="; \
	$(MAKE) $(IN_SERVER) verify || fail=1; \
	for target in ui-test spec-lint player-lint player-test; do \
		echo "==================== make $$target ===================="; \
		$(MAKE) --no-print-directory $$target || fail=1; \
	done; \
	if [ $$fail -ne 0 ]; then \
		echo "❌ verify failed (see above)"; \
		exit 1; \
	fi; \
	echo "✅ verify passed"

coverage-report: ## generate a coverage report
	@$(MAKE) $(IN_SERVER) coverage-report

#==========================================================================================
##@ Running
#==========================================================================================
run: ## start the GO service with the dev config (server/zarf/localdata/config.yaml)
	@$(MAKE) $(IN_SERVER) run

run-ui: package-ui run## build the UI and start the GO service

.PHONY: run-lab
run-lab: ## start the covergen cover-art tuning lab (dev-only; http://localhost:8099 — override with LAB_ADDR=:9000)
	@$(MAKE) $(IN_SERVER) run-lab

proxy: ## smoke-test proxy for auth proxy-header mode: make proxy USER=admin GROUP=aether-admin (GROUP optional)
	@$(MAKE) $(IN_SERVER) proxy

.PHONY: reset-data
reset-data: ## delete the local data dir (server/zarf/localdata/data; DATA_DIR is relative to server/, FORCE=1 skips the prompt)
	@$(MAKE) $(IN_SERVER) reset-data

.PHONY: sample-library
sample-library: ## download ~1 GB of freely licensed albums into server/zarf/locallibrary/sample-library (SAMPLE_LIBRARY_DIR is relative to server/)
	@$(MAKE) $(IN_SERVER) sample-library

#==========================================================================================
##@ Building
#==========================================================================================
package-ui: build-ui ## build the web and copy into Go package
	rm -rf ./server/app/spa/files/ui*
	mkdir -p ./server/app/spa/files/ui
	cp -r ./webui/dist/* ./server/app/spa/files/ui/
	touch ./server/app/spa/files/ui/.gitkeep
build-ui:
	@cd webui && \
	npm install && \
	npm run build

build: package-ui ## use goreleaser to build to current OS/Arch
	@goreleaser build --snapshot --clean --single-target

# The desktop player's app icon is the SPA favicon's artwork too: `tauri icon`
# renders its bundle set from server/zarf/icon/web/icon.svg. The android/ and
# ios/ sets and the 64x64.png it also writes are dropped — the player is not
# mobile-initialized and bundle.icon lists no 64px icon.
.PHONY: icons
icons: ## re-render the app icons from server/zarf/icon: the SPA set into webui/public, the desktop player's into player/src-tauri/icons (needs inkscape + imagemagick, and `make player-prepare`)
	@./server/zarf/icon/render.sh
	@echo "rendering icons into player/src-tauri/icons"
	@cd player && npx --no-install tauri icon ../server/zarf/icon/web/icon.svg
	@rm -rf player/src-tauri/icons/android player/src-tauri/icons/ios player/src-tauri/icons/64x64.png

#==========================================================================================
##@ Player
#==========================================================================================
.PHONY: player-prepare player-run player-test player-lint player-build
player-prepare: ## install the desktop player's tooling (tauri CLI) and the webui dependencies
	@$(MAKE) $(IN_PLAYER) prepare

player-run: ## run the desktop player in dev mode (vite dev server + app window; `make run` the server for data)
	@$(MAKE) $(IN_PLAYER) run

player-test: ## run the desktop player's rust tests
	@$(MAKE) $(IN_PLAYER) test

player-lint: ## format check + clippy for the desktop player
	@$(MAKE) $(IN_PLAYER) lint

player-build: ## build the desktop player bundles for this OS into player/target/release/bundle
	@$(MAKE) $(IN_PLAYER) build

#==========================================================================================
##@ Site
#==========================================================================================
site-serve: ## serve the website (site/) with live reload on http://localhost:1313/aether/ (needs hugo extended + go)
	@$(MAKE) $(IN_SITE) serve

site-build: ## build the website into site/public, as CI does
	@$(MAKE) $(IN_SITE) build

site-check: ## check the website builds (errors, links to missing pages) without leaving output
	@$(MAKE) $(IN_SITE) check

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
	@[ "${version}" ] || ( echo ">> version is not set, usage: make tag version=\"v1.2.3\" "; exit 1 )
	@echo "$(version)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$$' || ( echo ">> version \"$(version)\" must look like v1.2.3 (leading v): release.yml only publishes v*.*.* tags"; exit 1 )
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
