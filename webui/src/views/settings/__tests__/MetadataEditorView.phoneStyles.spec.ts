// @vitest-environment node
// Pin the phone header wrap off disk (scoped styles never apply under vue-test-utils).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const source = readFileSync(
    fileURLToPath(new URL('../MetadataEditorView.vue', import.meta.url)),
    'utf8'
)

describe('MetadataEditorView phone toolbar wrap', () => {
    const media = source.match(/@media \(max-width: 767\.98px\)\s*\{[\s\S]*?\n\}/)?.[0]

    it('has a phone media query', () => {
        expect(media).toBeTruthy()
    })

    it('wraps the toolbar on phone', () => {
        expect(media).toMatch(/\.editor-header\s*\{[^}]*flex-wrap:\s*wrap/)
    })
})
