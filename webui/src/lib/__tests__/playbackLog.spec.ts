import { describe, it, expect, beforeEach, vi } from 'vitest'

type Mod = typeof import('@/lib/playbackLog')
let mod: Mod

beforeEach(async () => {
    localStorage.clear()
    vi.resetModules()
    mod = await import('@/lib/playbackLog')
})

describe('playbackLog', () => {
    it('records entries oldest first and survives a reload', async () => {
        mod.logPlayback('load', 'tr-1')
        mod.logPlayback('ended')
        expect(mod.playbackLog.value.map((e) => e.event)).toEqual(['load', 'ended'])
        expect(mod.playbackLog.value[0]!.detail).toBe('tr-1')
        expect(mod.playbackLog.value[1]).not.toHaveProperty('detail')

        vi.resetModules()
        const reloaded = await import('@/lib/playbackLog')
        expect(reloaded.playbackLog.value.map((e) => e.event)).toEqual(['load', 'ended'])
    })

    it('keeps only the latest entries', () => {
        for (let i = 0; i < 350; i++) mod.logPlayback('tick', String(i))
        expect(mod.playbackLog.value).toHaveLength(300)
        expect(mod.playbackLog.value[0]!.detail).toBe('50')
        expect(mod.playbackLog.value.at(-1)!.detail).toBe('349')
    })

    it('clears', () => {
        mod.logPlayback('load')
        mod.clearPlaybackLog()
        expect(mod.playbackLog.value).toEqual([])
        expect(JSON.parse(localStorage.getItem('aether:playbackLog')!)).toEqual([])
    })

    it('starts empty from unreadable storage', async () => {
        localStorage.setItem('aether:playbackLog', '{not json')
        vi.resetModules()
        const reloaded = await import('@/lib/playbackLog')
        expect(reloaded.playbackLog.value).toEqual([])
    })

    it('formats one line per entry', () => {
        const at = new Date(2026, 10, 10, 9, 5, 7, 42).getTime()
        expect(
            mod.formatPlaybackLog([
                { at, event: 'error', detail: 'tr-1 code 2' },
                { at, event: 'page-hidden' }
            ])
        ).toBe('2026-11-10 09:05:07.042 error tr-1 code 2\n2026-11-10 09:05:07.042 page-hidden')
    })
})
