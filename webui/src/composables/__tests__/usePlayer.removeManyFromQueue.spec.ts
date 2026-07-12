import { describe, it, expect, beforeEach } from 'vitest'
import { usePlayer } from '@/composables/usePlayer'
import type { Song } from '@/types/subsonic'

const song = (id: string): Song => ({ id, title: `Song ${id}` })

describe('usePlayer.removeManyFromQueue', () => {
    let player: ReturnType<typeof usePlayer>

    beforeEach(() => {
        player = usePlayer()
        player.clearQueue()
        player.addMultipleToQueue([song('A'), song('B'), song('C'), song('D'), song('E')])
        player.currentIndex.value = 2 // C is current
    })

    it('removes several tracks at once', () => {
        player.removeManyFromQueue([0, 3]) // drop A and D
        expect(player.queue.value.map((s) => s.id)).toEqual(['B', 'C', 'E'])
    })

    it('keeps currentIndex on the same song when removing rows before it', () => {
        const current = player.queue.value[player.currentIndex.value] // C
        player.removeManyFromQueue([0, 1]) // [C,D,E]
        expect(player.currentIndex.value).toBe(0)
        expect(player.queue.value[player.currentIndex.value]).toBe(current)
    })

    it('keeps currentIndex on the same song when removing rows after it', () => {
        const current = player.queue.value[player.currentIndex.value] // C
        player.removeManyFromQueue([3, 4]) // [A,B,C]
        expect(player.currentIndex.value).toBe(2)
        expect(player.queue.value[player.currentIndex.value]).toBe(current)
    })

    it('keeps currentIndex on the same song when removing rows on both sides', () => {
        const current = player.queue.value[player.currentIndex.value] // C
        player.removeManyFromQueue([1, 4]) // [A,C,D]
        expect(player.currentIndex.value).toBe(1)
        expect(player.queue.value[player.currentIndex.value]).toBe(current)
    })

    it('does nothing when no indices are given', () => {
        const before = player.queue.value.map((s) => s.id)
        player.removeManyFromQueue([])
        expect(player.queue.value.map((s) => s.id)).toEqual(before)
        expect(player.currentIndex.value).toBe(2)
    })

    it('advances to the next surviving track when the current track is removed', () => {
        player.removeManyFromQueue([2]) // drop the current track C → [A,B,D,E]
        expect(player.queue.value.map((s) => s.id)).toEqual(['A', 'B', 'D', 'E'])
        expect(player.currentIndex.value).toBe(2)
        expect(player.queue.value[player.currentIndex.value].id).toBe('D')
    })

    it('lands on the next survivor when the current track and its neighbours go', () => {
        player.removeManyFromQueue([2, 3]) // drop C and D → [A,B,E]
        expect(player.queue.value.map((s) => s.id)).toEqual(['A', 'B', 'E'])
        expect(player.currentIndex.value).toBe(2)
        expect(player.queue.value[player.currentIndex.value].id).toBe('E')
    })

    it('clamps to the new last track when the removed current was at the end', () => {
        player.currentIndex.value = 4 // E is current
        player.removeManyFromQueue([4]) // → [A,B,C,D]
        expect(player.currentIndex.value).toBe(3)
        expect(player.queue.value[player.currentIndex.value].id).toBe('D')
    })

    it('clears the queue when every track is removed', () => {
        player.removeManyFromQueue([0, 1, 2, 3, 4])
        expect(player.queue.value).toEqual([])
        expect(player.currentIndex.value).toBe(0)
        expect(player.currentTrack.value).toBeNull()
    })
})
