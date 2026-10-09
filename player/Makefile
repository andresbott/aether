# Desktop player (Tauri 2). The window it opens is the shared SPA at ../webui —
# there is no vendored copy here. `npx tauri dev|build` run the webui's own dev
# server / production build through tauri.conf.json's before*Command hooks, so
# both expect `webui/node_modules` to be installed (`make prepare`).
WEBUI_DIST := ../webui/dist

# Debian/Ubuntu package names; the same list is in .github/workflows/player.yml,
# release.yml and docs/agents/player/testing.md — keep them in sync.
LINUX_PKGS := libwebkit2gtk-4.1-dev build-essential curl wget file libxdo-dev libssl-dev libayatana-appindicator3-dev librsvg2-dev libasound2-dev libdbus-1-dev

default: help

prepare: ## install the tauri CLI (npm, here) and the webui's dependencies
	@npm install
	@npm --prefix ../webui install

# The build scripts of webkit2gtk-sys, libappindicator-sys, alsa-sys and
# libdbus-sys probe pkg-config for these; a missing one fails deep inside
# cargo with a less helpful message than this.
check-deps: ## verify the native build prerequisites (Linux; prints the apt line when something is missing)
	@if [ "$$(uname -s)" = "Linux" ]; then \
		missing=0; \
		for pc in webkit2gtk-4.1 ayatana-appindicator3-0.1 alsa dbus-1; do \
			pkg-config --exists $$pc 2>/dev/null || { echo "missing development library: $$pc"; missing=1; }; \
		done; \
		if [ $$missing -ne 0 ]; then \
			echo "install the Tauri build prerequisites, e.g. on Debian/Ubuntu:"; \
			echo "  sudo apt-get install -y $(LINUX_PKGS)"; \
			exit 1; \
		fi; \
	fi

# tauri::generate_context! embeds frontendDist at compile time and refuses a
# missing directory, so cargo needs a webui build in place. Built here once if
# absent; it is not refreshed when stale (`make build-ui` at the root does that,
# and `tauri build` always rebuilds it).
$(WEBUI_DIST)/index.html:
	@$(MAKE) --no-print-directory -C .. build-ui

webui-dist: $(WEBUI_DIST)/index.html

run: check-deps ## run the app in dev mode (vite dev server + the app window; `make run` the server on :8075 for data)
	@npx tauri dev

test: check-deps webui-dist ## run the rust tests (cargo test --workspace)
	@cargo test --workspace

lint: check-deps webui-dist ## format check + clippy (-D warnings)
	@cargo fmt --all -- --check
	@cargo clippy --workspace -- -D warnings

fmt: ## format rust code
	@cargo fmt --all

build: check-deps ## build production bundles into target/release/bundle (runs the webui production build first)
	@npx tauri build

clean: ## remove the cargo build output
	@cargo clean

#==========================================================================================
#  Help
#==========================================================================================
.PHONY: default prepare check-deps webui-dist run test lint fmt build clean help
help: # Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
