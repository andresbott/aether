import { describe, it, expect, beforeEach } from 'vitest'
import { usePlayer } from '@/composables/usePlayer'
import type { Song } from '@/types/subsonic'

const song = (id: string): Song => ({ id, title: `Song ${id}` })

describe('usePlayer.insertIntoQueue', () => {
    let player: ReturnType<typeof usePlayer>

    beforeEach(() => {
        player = usePlayer()
        player.clearQueue()
        player.addMultipleToQueue([song('A'), song('B'), song('C')])
        player.currentIndex.value = 1 // B is current
    })

    it('inserts a block at the target index', () => {
        player.insertIntoQueue([song('X'), song('Y')], 2)
        expect(player.queue.value.map((s) => s.id)).toEqual(['A', 'B', 'X', 'Y', 'C'])
    })

    it('appends when the target is at or past the end', () => {
        player.insertIntoQueue([song('X')], 99)
        expect(player.queue.value.map((s) => s.id)).toEqual(['A', 'B', 'C', 'X'])
    })

    it('shifts currentIndex when inserting before the current track', () => {
        const current = player.queue.value[player.currentIndex.value]
        player.insertIntoQueue([song('X'), song('Y')], 0)
        expect(player.queue.value[player.currentIndex.value]).toBe(current)
        expect(player.currentIndex.value).toBe(3) // B moved from 1 → 3
    })

    it('leaves currentIndex when inserting after the current track', () => {
        player.insertIntoQueue([song('X')], 3)
        expect(player.currentIndex.value).toBe(1)
    })

    it('does nothing for an empty song list', () => {
        player.insertIntoQueue([], 0)
        expect(player.queue.value.map((s) => s.id)).toEqual(['A', 'B', 'C'])
        expect(player.currentIndex.value).toBe(1)
    })

    it('fills an empty queue', () => {
        player.clearQueue()
        player.insertIntoQueue([song('X'), song('Y')], 0)
        expect(player.queue.value.map((s) => s.id)).toEqual(['X', 'Y'])
    })
})
