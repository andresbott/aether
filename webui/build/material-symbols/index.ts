import type { Plugin } from 'vite'
import { loadIconSet, type IconSet } from './iconset'
import { scanDir, buildIconCss } from './iconCss'
import { buildCatalogue, type Catalogue } from './catalogue'
import { LIBRARY_ICON_DIR } from '../../src/lib/libraryIcons'

const CSS_ID = 'virtual:material-symbols.css'
const CATALOGUE_ID = 'virtual:material-symbols/catalogue'
// Not \0-prefixed: the id must end in .css for Vite's CSS pipeline to take it
// (UnoCSS resolves its virtual sheet the same way).
const RESOLVED_CSS = '/__material-symbols.css'
const RESOLVED_CATALOGUE = '\0material-symbols-catalogue'

/**
 * Material Symbols (Rounded), self-hosted: CSS-mask classes for the icons the
 * source names (ms-<name> filled, mso-<name> outlined), one SVG file per
 * catalogue icon for runtime-chosen library icons, and the picker's name
 * list. Everything comes from node_modules — no network, ever.
 */
export function materialSymbols(opts: { srcDir: string }): Plugin {
    let set: IconSet | undefined
    let catalogue: Catalogue | undefined
    const iconSet = () => (set ??= loadIconSet())
    const getCatalogue = () => (catalogue ??= buildCatalogue(iconSet()))

    return {
        name: 'aether:material-symbols',
        resolveId(id) {
            if (id === CSS_ID) return RESOLVED_CSS
            if (id === CATALOGUE_ID) return RESOLVED_CATALOGUE
        },
        load(id) {
            if (id === RESOLVED_CSS) return buildIconCss(iconSet(), scanDir(opts.srcDir))
            if (id === RESOLVED_CATALOGUE) return `export default ${JSON.stringify(getCatalogue().names)}`
        },
        configureServer(server) {
            server.middlewares.use(`/${LIBRARY_ICON_DIR}`, (req, res, next) => {
                const m = /^\/([a-z0-9_]+)\.svg$/.exec(req.url ?? '')
                const svg = m && getCatalogue().svgs.get(m[1])
                if (!svg) return next()
                res.setHeader('Content-Type', 'image/svg+xml')
                res.end(svg)
            })
            // A new icon class in any source file changes the sheet.
            const refresh = (file: string) => {
                if (!file.startsWith(opts.srcDir)) return
                const mod = server.moduleGraph.getModuleById(RESOLVED_CSS)
                if (mod) void server.reloadModule(mod)
            }
            server.watcher.on('change', refresh)
            server.watcher.on('add', refresh)
        },
        generateBundle() {
            for (const [name, svg] of getCatalogue().svgs) {
                this.emitFile({ type: 'asset', fileName: `${LIBRARY_ICON_DIR}/${name}.svg`, source: svg })
            }
        }
    }
}
