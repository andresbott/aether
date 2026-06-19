import { describe, it, expect, beforeEach } from 'vitest'
import { usePlayer } from '@/composables/usePlayer'
import type { Song } from '@/types/subsonic'

const song = (id: string): Song => ({ id, title: `Song ${id}` })

describe('usePlayer.moveInQueue', () => {
    let player: ReturnType<typeof usePlayer>

    beforeEach(() => {
        player = usePlayer()
        player.clearQueue()
        player.addMultipleToQueue([song('A'), song('B'), song('C'), song('D')])
        player.currentIndex.value = 1 // B is current
    })

    it('reorders the queue', () => {
        player.moveInQueue([3], 0) // move D to the front
        expect(player.queue.value.map((s) => s.id)).toEqual(['D', 'A', 'B', 'C'])
    })

    it('keeps currentIndex pointing at the same song after a move', () => {
        const current = player.queue.value[player.currentIndex.value]
        player.moveInQueue([3], 0) // [D,A,B,C] → B now at index 2
        expect(player.currentIndex.value).toBe(2)
        expect(player.queue.value[player.currentIndex.value]).toBe(current)
    })

    it('does nothing when no indices are given', () => {
        const before = player.queue.value.map((s) => s.id)
        player.moveInQueue([], 0)
        expect(player.queue.value.map((s) => s.id)).toEqual(before)
        expect(player.currentIndex.value).toBe(1)
    })
})
