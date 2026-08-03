import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import ShortcutHelpOverlay from '@/components/layout/ShortcutHelpOverlay.vue'
import { useShortcutHelp } from '@/composables/useShortcutHelp'
import { VISIBLE_SHORTCUTS, OVERLAY_SHORTCUTS } from '@/utils/shortcuts'

// The overlay measures the real controls, and jsdom gives every element a
// zero-size rect — which the component reads as "not visible" and skips. So
// plant stand-ins carrying the data-shortcut anchors with stubbed geometry,
// exactly as the player bar would.
const planted: HTMLElement[] = []

const plantAnchor = (anchor: string, rect: Partial<DOMRect>): HTMLElement => {
    const el = document.createElement('button')
    el.dataset.shortcut = anchor
    document.body.appendChild(el)
    // right/bottom are DERIVED, never passed in: a real rect always has
    // right === left + width, and a stub that can disagree tests nothing.
    const box = { left: 0, top: 0, width: 40, height: 40, ...rect }
    el.getBoundingClientRect = () =>
        ({ ...box, right: box.left + box.width, bottom: box.top + box.height }) as DOMRect
    planted.push(el)
    return el
}

const mountOverlay = () => mount(ShortcutHelpOverlay, { attachTo: document.body })

beforeEach(() => {
    useShortcutHelp().open.value = true
    window.innerWidth = 1400
    window.innerHeight = 900
})

afterEach(() => {
    useShortcutHelp().close()
    for (const el of planted) el.remove()
    planted.length = 0
})

describe('ShortcutHelpOverlay visibility', () => {
    it('renders nothing while the help flag is down', () => {
        useShortcutHelp().close()
        const w = mountOverlay()
        expect(w.find('.shortcut-overlay').exists()).toBe(false)
    })

    it('renders the dimmed backdrop while open', () => {
        const w = mountOverlay()
        expect(w.find('.shortcut-overlay').exists()).toBe(true)
    })

    it('closes when the backdrop is clicked', async () => {
        const help = useShortcutHelp()
        const w = mountOverlay()
        await w.find('.shortcut-overlay').trigger('click')
        expect(help.open.value).toBe(false)
    })
})

describe('ShortcutHelpOverlay anchored badges', () => {
    it('pins a badge over each visible control it finds', async () => {
        plantAnchor('play-pause', { left: 600, top: 820, width: 44, height: 44 })
        plantAnchor('mute', { left: 1100, top: 825, width: 30, height: 30 })
        const w = mountOverlay()
        await w.vm.$nextTick()

        const badges = w.findAll('.shortcut-badge')
        expect(badges).toHaveLength(2)
        expect(badges.map((b) => b.text())).toContain('Space')
        expect(badges.map((b) => b.text())).toContain('M')
    })

    // Positions come from getBoundingClientRect, not from per-breakpoint CSS, so
    // every screen size is handled by the same code path. The badge is centred on
    // its control and sits above it.
    it('centres each badge on its control and places it above', async () => {
        plantAnchor('play-pause', { left: 600, top: 820, width: 44, height: 44 })
        const w = mountOverlay()
        await w.vm.$nextTick()

        const style = w.find('.shortcut-badge').attributes('style') ?? ''
        // centre 622px, so a badge of any width is placed by its own transform.
        expect(style).toContain('left: 622px')
        // Above the control's top edge (820), never overlapping it.
        const top = Number(/top:\s*(-?\d+(?:\.\d+)?)px/.exec(style)?.[1])
        expect(top).toBeLessThan(820)
    })

    // The volume rail and speaker are display:none under 768px, where jsdom's
    // zero rect stands in for "not laid out" — the badge must be skipped rather
    // than pinned to 0,0.
    it('skips controls that are not laid out', async () => {
        plantAnchor('play-pause', { left: 600, top: 820, width: 44, height: 44 })
        plantAnchor('mute', { left: 0, top: 0, width: 0, height: 0 })
        const w = mountOverlay()
        await w.vm.$nextTick()

        const badges = w.findAll('.shortcut-badge')
        expect(badges).toHaveLength(1)
        expect(badges[0].text()).toBe('Space')
    })

    it('keeps a badge inside the viewport when its control sits at the edge', async () => {
        window.innerWidth = 400
        plantAnchor('queue', { left: 370, top: 820, width: 30, height: 30 })
        const w = mountOverlay()
        await w.vm.$nextTick()

        const style = w.find('.shortcut-badge').attributes('style') ?? ''
        const left = Number(/left:\s*(-?\d+(?:\.\d+)?)px/.exec(style)?.[1])
        expect(left).toBeLessThanOrEqual(400)
        expect(left).toBeGreaterThan(0)
    })

    it('re-measures when the window is resized', async () => {
        const el = plantAnchor('play-pause', { left: 600, top: 820, width: 44, height: 44 })
        const w = mountOverlay()
        await w.vm.$nextTick()
        expect(w.find('.shortcut-badge').attributes('style')).toContain('left: 622px')

        el.getBoundingClientRect = () =>
            ({ left: 300, top: 820, width: 44, height: 44, right: 344, bottom: 864 }) as DOMRect
        window.dispatchEvent(new Event('resize'))
        await w.vm.$nextTick()
        expect(w.find('.shortcut-badge').attributes('style')).toContain('left: 322px')
    })

    it('makes the badges non-interactive so they cannot be clicked', () => {
        plantAnchor('play-pause', { left: 600, top: 820, width: 44, height: 44 })
        const w = mountOverlay()
        expect(w.find('.shortcut-badges').attributes('aria-hidden')).toBe('true')
    })

    // Volume up/down and seek forward/back drive one control apiece, so their
    // badge has to carry both keys — a lone "+" over the rail would teach half
    // the binding and leave the other half only in the panel.
    it('shows both directions on the badge of a shared control', async () => {
        plantAnchor('volume', { left: 1150, top: 828, width: 150, height: 5 })
        plantAnchor('progress', { left: 500, top: 855, width: 400, height: 6 })
        const w = mountOverlay()
        await w.vm.$nextTick()

        // Each key is its own span inside the badge, so assert on the spans —
        // `.text()` would run them together.
        const pairs = w
            .findAll('.shortcut-badge')
            .map((b) => b.findAll('.shortcut-badge-key').map((k) => k.text()))
        // Read the way they sit on the control: back before forward, up before down.
        expect(pairs).toContainEqual(['←', '→'])
        expect(pairs).toContainEqual(['↑', '↓'])
    })

    it('gives a shared control exactly one badge', async () => {
        plantAnchor('volume', { left: 1150, top: 828, width: 150, height: 5 })
        const w = mountOverlay()
        await w.vm.$nextTick()
        expect(w.findAll('.shortcut-badge')).toHaveLength(1)
    })

    // The rails are wide (the progress bar spans the centre column), so centring
    // the badge on the control is what puts it "close to" the bar it drives.
    it('centres a rail badge on the rail and lifts it clear', async () => {
        plantAnchor('progress', { left: 500, top: 855, width: 400, height: 6 })
        const w = mountOverlay()
        await w.vm.$nextTick()

        const style = w.find('.shortcut-badge').attributes('style') ?? ''
        expect(style).toContain('left: 700px')
        const top = Number(/top:\s*(-?\d+(?:\.\d+)?)px/.exec(style)?.[1])
        expect(top).toBeLessThan(855)
    })
})

describe('ShortcutHelpOverlay list panel', () => {
    it('lists every shortcut that has no on-screen control', async () => {
        plantAnchor('play-pause', { left: 600, top: 820, width: 44, height: 44 })
        const w = mountOverlay()
        await w.vm.$nextTick()

        const text = w.find('.shortcut-panel').text()
        // Not badged at this width (no progress anchor planted), so the panel
        // carries the keys instead.
        expect(text).toContain('Seek forward 5s')
        // Already carrying a badge over its control, so not repeated here.
        expect(text).not.toContain('Play / pause')
    })

    // You just pressed `?` to open this, so a row telling you about `?` teaches
    // nothing. It stays in the Settings list, which is the full reference.
    it('never lists the help key itself', async () => {
        const w = mountOverlay()
        await w.vm.$nextTick()
        expect(w.find('.shortcut-panel').text()).not.toContain('This shortcut list')
    })

    // With every control on screen, nothing is left to list — the panel must then
    // drop its list entirely rather than render an empty one under the title.
    it('renders no list when every shortcut is badged', async () => {
        for (const anchor of new Set(OVERLAY_SHORTCUTS.map((s) => s.anchor).filter(Boolean))) {
            plantAnchor(anchor as string, { left: 400, top: 400, width: 40, height: 40 })
        }
        const w = mountOverlay()
        await w.vm.$nextTick()

        expect(w.findAll('.shortcut-row')).toHaveLength(0)
        // The Esc hint still stands, so the panel is never an empty box.
        expect(w.find('.shortcut-panel').text().toLowerCase()).toContain('esc')
    })

    // Search is anchored on its sidebar entry now, so its badge is the teacher and
    // the panel must not repeat the key.
    it('drops Search from the panel once its sidebar badge is up', async () => {
        plantAnchor('search', { left: 20, top: 200, width: 180, height: 40 })
        const w = mountOverlay()
        await w.vm.$nextTick()

        expect(w.findAll('.shortcut-badge').map((b) => b.text())).toContain('S')
        expect(w.find('.shortcut-panel').text()).not.toContain('Search')
    })

    // Both halves of a shared binding are badged together, so neither may also
    // appear in the panel — that would list the same key twice.
    it('drops both directions of a shared binding from the panel', async () => {
        plantAnchor('volume', { left: 1150, top: 828, width: 150, height: 5 })
        const w = mountOverlay()
        await w.vm.$nextTick()

        const text = w.find('.shortcut-panel').text()
        expect(text).not.toContain('Volume up')
        expect(text).not.toContain('Volume down')
    })
})

// The player-bar badges float above their control, where there is empty dimmed
// space. A sidebar nav entry has no room above it — the entry above is another
// nav item — but plenty to its right, so those badges sit beside the control
// instead. The side is a property of the registry entry, not of the overlay.
describe('ShortcutHelpOverlay badge placement', () => {
    it('places a side-placed badge to the right of its control', async () => {
        plantAnchor('now-playing', { left: 20, top: 120, width: 180, height: 40 })
        const w = mountOverlay()
        await w.vm.$nextTick()

        const badge = w.find('.shortcut-badge')
        const style = badge.attributes('style') ?? ''
        // Just past the control's right edge (200), not centred on it (110).
        const left = Number(/left:\s*(-?\d+(?:\.\d+)?)px/.exec(style)?.[1])
        expect(left).toBeGreaterThanOrEqual(200)
        // Vertically centred on the entry (120..160), not lifted above it.
        const top = Number(/top:\s*(-?\d+(?:\.\d+)?)px/.exec(style)?.[1])
        expect(top).toBeGreaterThan(120)
        expect(top).toBeLessThan(160)
    })

    // The transform is what does the actual placing, so a side badge must not
    // carry the above-placement one (which would lift it clear of its own row).
    it('marks a side-placed badge so its transform differs from a floating one', async () => {
        plantAnchor('now-playing', { left: 20, top: 120, width: 180, height: 40 })
        plantAnchor('play-pause', { left: 600, top: 820, width: 44, height: 44 })
        const w = mountOverlay()
        await w.vm.$nextTick()

        const badges = w.findAll('.shortcut-badge')
        const side = badges.find((b) => b.text() === 'C')
        const above = badges.find((b) => b.text() === 'Space')
        expect(side?.classes()).toContain('shortcut-badge--side')
        expect(above?.classes()).not.toContain('shortcut-badge--side')
    })

    // With the bar collapsed on a narrow screen there is no badge to teach the
    // key, so the panel has to carry it instead.
    it('falls back to listing a shortcut whose control is hidden', async () => {
        const w = mountOverlay()
        await w.vm.$nextTick()
        expect(w.find('.shortcut-panel').text()).toContain('Play / pause')
    })

    it('never lists the overlay-only Escape row', async () => {
        const w = mountOverlay()
        await w.vm.$nextTick()
        expect(w.find('.shortcut-panel').text()).not.toContain('Close this overlay')
    })

    it('accounts for every navigation shortcut once its entry is on screen', async () => {
        for (const anchor of ['now-playing', 'library', 'radio', 'genres', 'playlists', 'search']) {
            plantAnchor(anchor, { left: 20, top: 120, width: 180, height: 40 })
        }
        const w = mountOverlay()
        await w.vm.$nextTick()

        const keys = w.findAll('.shortcut-badge').map((b) => b.text())
        for (const key of ['C', 'D', 'R', 'G', 'P', 'S']) expect(keys).toContain(key)
    })

    it('shows a hint that Escape closes the overlay', () => {
        const w = mountOverlay()
        expect(w.text().toLowerCase()).toContain('esc')
    })

    // Nothing the overlay is responsible for may go unshown: every OVERLAY_SHORTCUTS
    // entry is either badged on its control or listed in the panel. `?` is excluded
    // by construction (OVERLAY_SHORTCUTS drops it), which is what this asserts is
    // the ONLY omission.
    it('accounts for every overlay shortcut across badges and panel', async () => {
        // One stand-in per distinct anchor, since several actions share one.
        for (const anchor of new Set(OVERLAY_SHORTCUTS.map((s) => s.anchor).filter(Boolean))) {
            plantAnchor(anchor as string, { left: 400, top: 820, width: 40, height: 40 })
        }
        const w = mountOverlay()
        await w.vm.$nextTick()

        const shown = w.text()
        for (const s of OVERLAY_SHORTCUTS) {
            // Either its badge or its panel row must mention it.
            expect(shown).toContain(s.anchor ? s.keys[0] : s.label)
        }
        // The settings list carries `?`; the overlay deliberately does not.
        expect(VISIBLE_SHORTCUTS.some((s) => s.action === 'help')).toBe(true)
        expect(OVERLAY_SHORTCUTS.some((s) => s.action === 'help')).toBe(false)
    })
})
