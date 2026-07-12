import { describe, it, expect, vi, beforeAll } from 'vitest'
import { mount } from '@vue/test-utils'
import type { InternetRadioStation } from '@/types/subsonic'
import type { RadioStationPrefill } from '@/types/radiobrowser'
import StationEditPanel from '@/views/settings/radio-stations/StationEditPanel.vue'

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { getCoverArtUrl: (id: string) => `/cover/${id}` }
}))

beforeAll(() => {
    // jsdom has no object-URL support; the panel previews picked covers with it.
    globalThis.URL.createObjectURL = vi.fn(() => 'blob:mock')
    globalThis.URL.revokeObjectURL = vi.fn()
})

const stubs = {
    InputText: {
        props: ['modelValue'],
        template:
            '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
    },
    // inheritAttrs:false so the parent's @click doesn't ALSO fall through as a
    // native listener (double-firing); label/aria-label bound explicitly.
    Button: {
        props: ['label', 'disabled'],
        inheritAttrs: false,
        template:
            '<button :disabled="disabled" :aria-label="$attrs[\'aria-label\']" @click="$emit(\'click\')">{{ label }}</button>'
    },
    FileUpload: { name: 'FileUpload', emits: ['select'], template: '<div class="file-upload" />' },
    Message: { template: '<div class="p-message"><slot /></div>' }
}

const station: InternetRadioStation = {
    id: 'r-1',
    name: 'BBC Radio 1',
    streamUrl: 'http://bbc/stream',
    homepageUrl: 'http://bbc/home',
    coverArt: 'r-1'
}

const mountPanel = (
    props: Partial<{
        station: InternetRadioStation | null
        adding: boolean
        submitting: boolean
        initial: RadioStationPrefill | null
    }>
) =>
    mount(StationEditPanel, {
        props: { station: null, adding: false, submitting: false, ...props },
        global: { stubs }
    })

const clickButton = async (wrapper: ReturnType<typeof mountPanel>, label: string) => {
    const btn = wrapper.findAll('button').find((b) => b.text() === label)
    await btn!.trigger('click')
}

describe('StationEditPanel', () => {
    it('prompts to pick or add a station when nothing is selected', () => {
        const w = mountPanel({ station: null, adding: false })
        expect(w.text()).toContain('Select a station')
        expect(w.find('input.field-name').exists()).toBe(false)
    })

    it('add mode: Create is disabled until a name and stream URL are given, then emits trimmed input', async () => {
        const w = mountPanel({ station: null, adding: true })
        const createBtn = () => w.findAll('button').find((b) => b.text() === 'Create')!
        expect(createBtn().attributes('disabled')).toBeDefined()

        await w.find('input.field-name').setValue('  Jazz FM  ')
        await w.find('input.field-stream-url').setValue('  http://jazz  ')
        expect(createBtn().attributes('disabled')).toBeUndefined()

        await createBtn().trigger('click')
        const input = w.emitted('save')?.[0]?.[0] as Record<string, unknown>
        expect(input.name).toBe('Jazz FM')
        expect(input.streamUrl).toBe('http://jazz')
        expect(input.homepageUrl).toBeUndefined()
        expect(input.coverFile).toBeUndefined()
    })

    it('edit mode: pre-fills fields and emits the edited input on Save', async () => {
        const w = mountPanel({ station, adding: false })
        expect((w.find('input.field-name').element as HTMLInputElement).value).toBe('BBC Radio 1')
        expect((w.find('input.field-stream-url').element as HTMLInputElement).value).toBe(
            'http://bbc/stream'
        )

        await w.find('input.field-name').setValue('BBC R1')
        await clickButton(w, 'Save')
        const input = w.emitted('save')?.[0]?.[0] as Record<string, unknown>
        expect(input.name).toBe('BBC R1')
        expect(input.streamUrl).toBe('http://bbc/stream')
        expect(input.homepageUrl).toBe('http://bbc/home')
    })

    it('edit mode: exposes a Delete action that emits delete', async () => {
        const w = mountPanel({ station, adding: false })
        await clickButton(w, 'Delete')
        expect(w.emitted('delete')).toHaveLength(1)
    })

    it('add mode: has no Delete action', () => {
        const w = mountPanel({ station: null, adding: true })
        expect(w.findAll('button').some((b) => b.text() === 'Delete')).toBe(false)
    })

    it('disables save when the name is cleared', async () => {
        const w = mountPanel({ station, adding: false })
        await w.find('input.field-name').setValue('')
        const saveBtn = w.findAll('button').find((b) => b.text() === 'Save')!
        expect(saveBtn.attributes('disabled')).toBeDefined()
    })

    it('includes a chosen cover file in the saved input', async () => {
        const w = mountPanel({ station: null, adding: true })
        await w.find('input.field-name').setValue('Jazz')
        await w.find('input.field-stream-url').setValue('http://jazz')

        const file = new File(['x'], 'cover.png', { type: 'image/png' })
        w.findComponent({ name: 'FileUpload' }).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()

        await clickButton(w, 'Create')
        const input = w.emitted('save')?.[0]?.[0] as Record<string, unknown>
        expect(input.coverFile).toBe(file)
    })

    it('stages a cover clear when the existing cover is removed', async () => {
        const w = mountPanel({ station, adding: false })
        await clickButton(w, 'Remove cover')
        await clickButton(w, 'Save')
        const input = w.emitted('save')?.[0]?.[0] as Record<string, unknown>
        expect(input.coverClear).toBe(true)
    })

    it('add mode: seeds fields and cover from an initial prefill', async () => {
        const file = new File(['x'], 'favicon.png', { type: 'image/png' })
        const initial: RadioStationPrefill = {
            name: 'Radio Paradise',
            streamUrl: 'http://rp/stream',
            homepageUrl: 'https://radioparadise.com',
            coverFile: file
        }
        const w = mountPanel({ station: null, adding: true, initial })

        expect((w.find('input.field-name').element as HTMLInputElement).value).toBe(
            'Radio Paradise'
        )
        expect((w.find('input.field-stream-url').element as HTMLInputElement).value).toBe(
            'http://rp/stream'
        )
        expect((w.find('input.field-homepage').element as HTMLInputElement).value).toBe(
            'https://radioparadise.com'
        )

        // The prefilled favicon flows through as the cover on Create.
        await clickButton(w, 'Create')
        const input = w.emitted('save')?.[0]?.[0] as Record<string, unknown>
        expect(input.name).toBe('Radio Paradise')
        expect(input.coverFile).toBe(file)
    })
})
