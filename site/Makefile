# Website targets (Hugo + the hugo-book theme module). The root Makefile
# delegates its site-* targets here. Needs Hugo extended >= 0.146 and Go, which
# Hugo uses to fetch the theme module.

THEME := github.com/alex-shpak/hugo-book

default: help

#==========================================================================================
##@ Running
#==========================================================================================
PORT ?= 1313
serve: ## serve the site with live reload on http://localhost:1313/aether/ (PORT=1414 to change)
	@hugo server --port $(PORT)

#==========================================================================================
##@ Building
#==========================================================================================
build: ## build the site into public/, as CI does
	@hugo --gc --minify

# Hugo writes its resource cache (resources/) and build lock next to the config
# whatever --destination says, so both are sent to the temp dir or turned off.
# Only stdout (the stats table) is dropped: errors are on stderr, which
# --quiet would hide too.
.PHONY: check
check: ## build into a temp dir and discard it: fails on errors and links to missing pages
	@out=$$(mktemp -d) && trap 'rm -rf "$$out"' EXIT && \
	HUGO_RESOURCEDIR="$$out/resources" hugo --gc --minify --noBuildLock \
		--destination "$$out/public" > /dev/null && echo "✅ site builds"

clean: ## delete the build output and Hugo's resource cache
	@rm -rf public resources .hugo_build.lock

#==========================================================================================
##@ Content
#==========================================================================================
new: ## create a page from the archetype: make new PAGE=docs/guides/my-page.md
	@[ "$(PAGE)" ] || ( echo ">> PAGE is not set, usage: make new PAGE=docs/guides/my-page.md"; exit 1 )
	@hugo new content "$(PAGE)"

#==========================================================================================
##@ Theme
#==========================================================================================
# The theme's release tags (v13, v12.0.0 without a /v12 module path) are not
# Go-module versions: `hugo mod get -u` would "upgrade" to its ancient v0.15.0
# tag. So a release is pinned by the commit its tag points to.
theme-update: ## pin the theme to a hugo-book release tag: make theme-update TAG=v13
	@[ "$(TAG)" ] || ( echo ">> TAG is not set, usage: make theme-update TAG=v13 (tags: https://github.com/alex-shpak/hugo-book/tags)"; exit 1 )
	@sha=$$(git ls-remote "https://$(THEME)" "refs/tags/$(TAG)" "refs/tags/$(TAG)^{}" | tail -n1 | cut -f1); \
	if [ -z "$$sha" ]; then echo ">> tag $(TAG) not found in $(THEME)"; exit 1; fi; \
	echo ">> $(THEME) $(TAG) = $$sha"; \
	hugo mod get "$(THEME)@$$sha" && hugo mod tidy

#==========================================================================================
#  Help
#==========================================================================================
.PHONY: help
help: # Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
