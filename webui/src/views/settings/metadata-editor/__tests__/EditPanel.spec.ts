import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import EditPanel from '@/views/settings/metadata-editor/EditPanel.vue'
import type { Track } from '@/types/metadata'

vi.mock('@/components/library/MusicBrainzArtistPicker.vue', () => ({
    default: { props: ['visible', 'artistName', 'currentMbid'], template: '<div />' }
}))

const stubs = {
    InputText: { props: ['modelValue'], template: '<input />' },
    InputNumber: { props: ['modelValue'], template: '<input />' },
    Checkbox: { props: ['modelValue'], template: '<input type="checkbox" />' },
    Chips: { props: ['modelValue'], template: '<div />' },
    Button: { props: ['label', 'disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>' }
}

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3', name: 'a.mp3', title: '', artists: ['X'], album_artists: [],
    album: '', year: 0, compilation: false, mb_artist_ids: [''], mb_album_artist_ids: [], ...over
})

describe('EditPanel MB-ID editing', () => {
    it('includes artist_mbids for a name whose MBID was set', async () => {
        const wrapper = mount(EditPanel, {
            props: { selection: [mkTrack()], isSaving: false },
            global: { stubs }
        })
        // Simulate the picker selecting an id for artist "X".
        ;(wrapper.vm as any).onArtistMbidSelect('X', 'id-x')
        await wrapper.vm.$nextTick()
        const saveButton = wrapper.findAll('button').find((b) => b.text() === 'Save')
        await saveButton!.trigger('click')
        const payload = wrapper.emitted('save')?.[0]?.[0] as any
        expect(payload.artist_mbids).toEqual({ X: 'id-x' })
    })
})
