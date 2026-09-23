import { describe, it, expect } from 'vitest'
import { mkdtempSync, writeFileSync, mkdirSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { materialSymbols } from '../index'

function fixture(files: Record<string, string>): string {
    const dir = mkdtempSync(join(tmpdir(), 'ms-'))
    for (const [rel, text] of Object.entries(files)) {
        mkdirSync(join(dir, rel, '..'), { recursive: true })
        writeFileSync(join(dir, rel), text)
    }
    return dir
}

type Hook = (this: unknown, ...a: unknown[]) => unknown
const call = (p: ReturnType<typeof materialSymbols>, hook: 'resolveId' | 'load', arg: string) =>
    (p[hook] as Hook).call({}, arg)

describe('materialSymbols plugin', () => {
    it('serves CSS for exactly the classes the source uses', () => {
        const src = fixture({
            'A.vue': '<i class="ms-play-arrow"></i>',
            '__tests__/A.spec.ts': "find('.ms-delete')"
        })
        const p = materialSymbols({ srcDir: src })
        const id = call(p, 'resolveId', 'virtual:material-symbols.css') as string
        expect(id).toMatch(/\.css$/)
        const css = call(p, 'load', id) as string
        expect(css).toContain('.ms-play-arrow{')
        expect(css).not.toContain('.ms-delete{')
    })

    it('serves the catalogue as an ES module', () => {
        const p = materialSymbols({ srcDir: fixture({}) })
        const id = call(p, 'resolveId', 'virtual:material-symbols/catalogue') as string
        const code = call(p, 'load', id) as string
        expect(code.startsWith('export default [')).toBe(true)
        expect(code).toContain('"queue_music"')
    })

    it('emits one SVG per catalogue icon into icons/ms', () => {
        const p = materialSymbols({ srcDir: fixture({}) })
        const emitted: { fileName: string; source: string }[] = []
        ;(p.generateBundle as Hook).call({ emitFile: (f: { fileName: string; source: string }) => emitted.push(f) })
        const folder = emitted.find((f) => f.fileName === 'icons/ms/folder.svg')
        expect(folder?.source).toMatch(/^<svg /)
        expect(emitted.length).toBeGreaterThan(3500)
    })
})
