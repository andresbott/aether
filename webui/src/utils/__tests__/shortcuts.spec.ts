import { describe, it, expect } from 'vitest'
import {
    SHORTCUTS,
    resolveShortcutAction,
    isTypingTarget,
    type ShortcutAction
} from '@/utils/shortcuts'

const press = (key: string, init: Partial<KeyboardEventInit> = {}): KeyboardEvent =>
    new KeyboardEvent('keydown', { key, ...init })

describe('resolveShortcutAction key map', () => {
    it.each<[string, ShortcutAction]>([
        [' ', 'play-pause'],
        ['n', 'next'],
        ['b', 'previous'],
        ['ArrowRight', 'seek-forward'],
        ['ArrowLeft', 'seek-back'],
        ['ArrowUp', 'volume-up'],
        ['ArrowDown', 'volume-down'],
        ['m', 'mute'],
        ['l', 'favorite'],
        ['q', 'queue'],
        ['c', 'now-playing'],
        ['d', 'library'],
        ['r', 'radio'],
        ['g', 'genres'],
        ['p', 'playlists'],
        ['s', 'search'],
        ['?', 'help'],
        ['Escape', 'close']
    ])('maps %s to %s', (key, action) => {
        expect(resolveShortcutAction(press(key))).toBe(action)
    })

    // Volume moved to the arrows to match the convention every web player uses
    // (YouTube: ←/→ seek, ↑/↓ volume). The old +/- spellings are gone, not kept
    // as aliases — one binding per action, discoverable from one badge.
    it('no longer binds the +/- keys', () => {
        for (const key of ['+', '-', '=', '_']) {
            expect(resolveShortcutAction(press(key))).toBeNull()
        }
    })

    it('is case-insensitive, so a stuck Caps Lock still works', () => {
        expect(resolveShortcutAction(press('N'))).toBe('next')
        expect(resolveShortcutAction(press('M'))).toBe('mute')
    })

    it('ignores keys that are not bound', () => {
        expect(resolveShortcutAction(press('z'))).toBeNull()
        expect(resolveShortcutAction(press('F5'))).toBeNull()
    })

    // Previous moved to `B` (as in "back"), which freed `P` for playlists — so `O`,
    // the stand-in playlists used while `P` was taken, is unbound again.
    it('no longer binds the O key', () => {
        expect(resolveShortcutAction(press('o'))).toBeNull()
    })

    // Search moved to `s`; `/` is no longer bound, so Firefox's quick-find gets it
    // back rather than being swallowed.
    it('no longer binds the / key', () => {
        expect(resolveShortcutAction(press('/'))).toBeNull()
    })
})

// Every binding is a bare key so it cannot collide with a browser shortcut
// (Ctrl+L/K/F/T/W, Alt+Arrow history, Cmd+anything). The flip side is that a
// modified press must fall through to the browser untouched.
describe('resolveShortcutAction modifier handling', () => {
    it.each(['ctrlKey', 'metaKey', 'altKey'] as const)('declines a %s press', (modifier) => {
        expect(resolveShortcutAction(press('n', { [modifier]: true }))).toBeNull()
        expect(resolveShortcutAction(press(' ', { [modifier]: true }))).toBeNull()
    })

    // Shift is exempt: `?` cannot be typed without it on most layouts.
    it('allows a shifted press, since ? is a shifted key', () => {
        expect(resolveShortcutAction(press('?', { shiftKey: true }))).toBe('help')
    })
})

describe('isTypingTarget', () => {
    const el = (tag: string, attrs: Record<string, string> = {}): HTMLElement => {
        const node = document.createElement(tag)
        for (const [k, v] of Object.entries(attrs)) node.setAttribute(k, v)
        return node
    }

    it.each(['input', 'textarea', 'select'])('claims a focused %s', (tag) => {
        expect(isTypingTarget(el(tag))).toBe(true)
    })

    it('claims a contenteditable host', () => {
        const host = el('div')
        // jsdom does not implement isContentEditable, so the attribute is what
        // the guard reads.
        host.setAttribute('contenteditable', 'true')
        expect(isTypingTarget(host)).toBe(true)
    })

    it('claims a node inside an editable host, not just the host itself', () => {
        const host = el('div', { contenteditable: 'true' })
        const inner = el('span')
        host.appendChild(inner)
        expect(isTypingTarget(inner)).toBe(true)
    })

    // The volume/seek rails are focusable role=slider elements with their own
    // arrow-key handling; taking their arrows away would break keyboard seeking.
    it('claims a focused slider handle', () => {
        expect(isTypingTarget(el('span', { role: 'slider' }))).toBe(true)
    })

    it('leaves ordinary elements alone', () => {
        expect(isTypingTarget(el('div'))).toBe(false)
        expect(isTypingTarget(el('button'))).toBe(false)
        expect(isTypingTarget(null)).toBe(false)
    })

    it('does not claim a contenteditable="false" host', () => {
        expect(isTypingTarget(el('div', { contenteditable: 'false' }))).toBe(false)
    })
})

describe('SHORTCUTS registry', () => {
    it('describes every action the matcher can return', () => {
        const listed = new Set(SHORTCUTS.map((s) => s.action))
        const resolvable: ShortcutAction[] = [
            'play-pause',
            'next',
            'previous',
            'seek-forward',
            'seek-back',
            'volume-up',
            'volume-down',
            'mute',
            'favorite',
            'queue',
            'now-playing',
            'library',
            'radio',
            'genres',
            'playlists',
            'search',
            'help',
            'close'
        ]
        for (const action of resolvable) expect(listed).toContain(action)
    })

    it('gives every entry a display label and a human description', () => {
        for (const entry of SHORTCUTS) {
            expect(entry.keys.length).toBeGreaterThan(0)
            expect(entry.label.length).toBeGreaterThan(0)
        }
    })

    // The overlay anchors a badge over the real control by this id; the settings
    // list renders the same rows. A typo'd anchor must not silently vanish, so
    // the anchored set is asserted here and PlayerControls carries the matching
    // data-shortcut attributes (see its spec).
    it('anchors exactly the actions that have a visible control', () => {
        const anchored = SHORTCUTS.filter((s) => s.anchor).map((s) => s.anchor)
        expect(anchored.sort()).toEqual(
            [
                'favorite',
                'genres',
                'library',
                'mute',
                'next',
                'now-playing',
                'play-pause',
                'playlists',
                'radio',
                'previous',
                // Both volume keys and both seek keys share one anchor apiece, so
                // the pair reads as one badge on the rail it drives.
                'progress',
                'progress',
                'queue',
                'search',
                'volume',
                'volume'
            ].sort()
        )
    })

    // A rail badge that showed only one arrow would teach half the binding, so
    // the two directions have to sit together on the control they share.
    it('pairs both directions of volume and seek on one anchor each', () => {
        const byAnchor = (anchor: string) =>
            SHORTCUTS.filter((s) => s.anchor === anchor).map((s) => s.action)
        // Listed in the order the badge shows them: `↑ ↓` and `← →`.
        expect(byAnchor('volume')).toEqual(['volume-up', 'volume-down'])
        expect(byAnchor('progress')).toEqual(['seek-back', 'seek-forward'])
    })

    // The badge and the settings list both render `keys`, so the arrows must be
    // spelled as glyphs — "ArrowUp" would be the DOM's name for the key, not a
    // label anyone wants to read on a badge.
    it('spells the volume keys as arrow glyphs', () => {
        const keys = (action: string) => SHORTCUTS.find((s) => s.action === action)?.keys
        expect(keys('volume-up')).toEqual(['↑'])
        expect(keys('volume-down')).toEqual(['↓'])
    })

    // A player-bar control has dimmed space above it; a sidebar nav entry has
    // another nav item there and room only to its right. Every nav shortcut is
    // anchored in the sidebar, so all of them are side-placed.
    it('places the sidebar badges beside their controls, and the rest above', () => {
        const side = SHORTCUTS.filter((s) => s.place === 'side').map((s) => s.action)
        expect(side.sort()).toEqual([
            'genres',
            'library',
            'now-playing',
            'playlists',
            'radio',
            'search'
        ])
    })

    // `?` is deliberately NOT listed in the overlay — you just pressed it to get
    // there, so a row for it teaches nothing. It stays in the Settings list, which
    // is the full reference.
    it('keeps the help key out of the overlay but in the settings list', () => {
        const help = SHORTCUTS.find((s) => s.action === 'help')
        expect(help?.overlayHidden).toBe(true)
        expect(help?.hidden).not.toBe(true)
    })

    // With sixteen bindings, a duplicate is easy to add and would silently make
    // one of the two actions unreachable — the KEY_MAP entry that loses just never
    // fires, and its badge would then teach a lie.
    it('gives every action its own key', () => {
        const seen = new Map<string, string>()
        for (const shortcut of SHORTCUTS) {
            for (const key of shortcut.keys) {
                expect(seen.get(key), `${key} is bound to two actions`).toBeUndefined()
                seen.set(key, shortcut.action)
            }
        }
    })

    // The other half of that: a displayed key must actually resolve to its action.
    // A registry row whose `keys` disagree with KEY_MAP renders a badge for a key
    // that does nothing.
    it('resolves every displayed key to the action it is listed under', () => {
        const asEventKey: Record<string, string> = {
            Space: ' ',
            Esc: 'Escape',
            '←': 'ArrowLeft',
            '→': 'ArrowRight',
            '↑': 'ArrowUp',
            '↓': 'ArrowDown'
        }
        for (const shortcut of SHORTCUTS) {
            const shown = shortcut.keys[0] as string
            const key = asEventKey[shown] ?? shown
            expect(
                resolveShortcutAction(press(key)),
                `${shown} does not run ${shortcut.action}`
            ).toBe(shortcut.action)
        }
    })

    it('binds nothing to a bare Ctrl/Cmd/Alt combination', () => {
        for (const entry of SHORTCUTS) {
            for (const key of entry.keys) {
                expect(key).not.toMatch(/ctrl|cmd|meta|alt/i)
            }
        }
    })
})
