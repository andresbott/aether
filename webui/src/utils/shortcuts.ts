// The single source of truth for the app's keyboard shortcuts: the key map the
// global listener matches against, the rows the settings list renders, and the
// badges the help overlay anchors over the player bar all read from SHORTCUTS.
// Adding a binding here is all that is needed for it to appear in both places.
//
// Every binding is a BARE key — no Ctrl/Cmd/Alt — so nothing can collide with a
// browser shortcut (Ctrl+L/K/F/T/W/N, Cmd+anything, Alt+Arrow history). The
// matcher declines any press carrying one of those modifiers. Shift is the one
// exception, because `?` cannot be typed without it on most layouts.
//
// The four arrows are the player's transport: ←/→ seek, ↑/↓ volume — the layout
// every web player uses (YouTube, Spotify's web app). That does mean ↑/↓ no
// longer scroll the page, since handled keys are preventDefault'ed; PageUp,
// PageDown, Home and End are deliberately left unbound as the scroll fallback.
// A press on a focused rail handle still reaches PrimeVue (see isTypingTarget),
// so keyboard seeking keeps working.

export type ShortcutAction =
    | 'play-pause'
    | 'next'
    | 'previous'
    | 'seek-forward'
    | 'seek-back'
    | 'volume-up'
    | 'volume-down'
    | 'mute'
    | 'favorite'
    | 'queue'
    | 'now-playing'
    | 'library'
    | 'radio'
    | 'genres'
    | 'playlists'
    | 'search'
    | 'help'
    | 'close'

// Where the help overlay pins this shortcut's badge. The value matches a
// `data-shortcut` attribute on the real control — in PlayerControls, or in the
// sidebar for the three nav shortcuts, which have no player-bar control — so the
// badge is positioned from the live layout rather than from per-breakpoint CSS.
//
// Two actions may share one anchor: volume up/down drive the same rail, as do
// seek forward/back, and their badge shows both keys. A lone arrow over the
// volume bar would teach half the binding.
export type ShortcutAnchor =
    | 'play-pause'
    | 'next'
    | 'previous'
    | 'mute'
    | 'volume'
    | 'progress'
    | 'favorite'
    | 'queue'
    | 'now-playing'
    | 'library'
    | 'radio'
    | 'genres'
    | 'playlists'
    | 'search'

// Where the badge sits relative to its control. `above` (the default) floats it
// in the dimmed space over the player bar; `side` puts it just past the control's
// right edge, for a sidebar nav entry whose neighbour above is another nav item.
export type ShortcutPlacement = 'above' | 'side'

export interface Shortcut {
    action: ShortcutAction
    /** Keys as typed by the user, for display. `keys[0]` is the primary badge. */
    keys: string[]
    /** What the shortcut does, in the imperative. */
    label: string
    anchor?: ShortcutAnchor
    place?: ShortcutPlacement
    /** Kept out of the settings list and the overlay's panel. */
    hidden?: boolean
    /**
     * Kept out of the OVERLAY only — still listed in Settings. For `?` itself:
     * you just pressed it to open the overlay, so a row for it teaches nothing
     * there, but the settings list is the full reference and must include it.
     */
    overlayHidden?: boolean
}

export const SHORTCUTS: Shortcut[] = [
    { action: 'play-pause', keys: ['Space'], label: 'Play / pause', anchor: 'play-pause' },
    { action: 'next', keys: ['N'], label: 'Next track', anchor: 'next' },
    // `B` as in "back": it leaves `P` to playlists, whose initial it is.
    { action: 'previous', keys: ['B'], label: 'Previous track', anchor: 'previous' },
    // Both seek keys anchor to the progress rail, and both volume keys to the
    // volume rail: the badge sits on the bar it scrubs, showing both directions.
    // Order matters — the badge renders the pair in registry order, so each pair
    // is listed the way it reads on the badge: `← →`, `+ −`.
    { action: 'seek-back', keys: ['←'], label: 'Seek back 5s', anchor: 'progress' },
    { action: 'seek-forward', keys: ['→'], label: 'Seek forward 5s', anchor: 'progress' },
    { action: 'volume-up', keys: ['↑'], label: 'Volume up', anchor: 'volume' },
    { action: 'volume-down', keys: ['↓'], label: 'Volume down', anchor: 'volume' },
    { action: 'mute', keys: ['M'], label: 'Mute / unmute', anchor: 'mute' },
    { action: 'favorite', keys: ['L'], label: 'Favorite current track', anchor: 'favorite' },
    { action: 'queue', keys: ['Q'], label: 'Show / hide the queue', anchor: 'queue' },
    // Every nav shortcut is anchored on its sidebar entry — the only control that
    // opens it — and placed beside it: the row above each is another nav item, not
    // empty space.
    {
        action: 'now-playing',
        keys: ['C'],
        label: 'Open Now Playing',
        anchor: 'now-playing',
        place: 'side'
    },
    // The library ROOT (no folderId): the cross-collection entry, which opens on
    // the Discover feed — hence `D`.
    { action: 'library', keys: ['D'], label: 'Open the library', anchor: 'library', place: 'side' },
    {
        action: 'playlists',
        keys: ['P'],
        label: 'Open playlists',
        anchor: 'playlists',
        place: 'side'
    },
    { action: 'genres', keys: ['G'], label: 'Open genres', anchor: 'genres', place: 'side' },
    { action: 'radio', keys: ['R'], label: 'Open radio', anchor: 'radio', place: 'side' },
    { action: 'search', keys: ['S'], label: 'Search', anchor: 'search', place: 'side' },
    // Listed in Settings but not in the overlay: pressing `?` is how you got there.
    { action: 'help', keys: ['?'], label: 'This shortcut list', overlayHidden: true },
    // Esc only does something while the overlay is up, so it would be noise in
    // the settings list.
    { action: 'close', keys: ['Esc'], label: 'Close this overlay', hidden: true }
]

/** The rows the settings list renders — the full reference. */
export const VISIBLE_SHORTCUTS = SHORTCUTS.filter((s) => !s.hidden)

/**
 * What the overlay may show: the settings rows minus `?` itself. Anything left
 * here that carries a badge is dropped again by the overlay at measure time, so
 * on a wide screen the panel's list can legitimately be empty.
 */
export const OVERLAY_SHORTCUTS = VISIBLE_SHORTCUTS.filter((s) => !s.overlayHidden)

// event.key → action. Letters are lowercased before lookup so Caps Lock does not
// disable every binding.
const KEY_MAP: Record<string, ShortcutAction> = {
    ' ': 'play-pause',
    n: 'next',
    b: 'previous',
    arrowright: 'seek-forward',
    arrowleft: 'seek-back',
    arrowup: 'volume-up',
    arrowdown: 'volume-down',
    m: 'mute',
    l: 'favorite',
    q: 'queue',
    c: 'now-playing',
    d: 'library',
    p: 'playlists',
    g: 'genres',
    r: 'radio',
    s: 'search',
    '?': 'help',
    escape: 'close'
}

export function resolveShortcutAction(event: KeyboardEvent): ShortcutAction | null {
    if (event.ctrlKey || event.metaKey || event.altKey) return null
    return KEY_MAP[event.key.toLowerCase()] ?? null
}

// A press is ignored when focus sits somewhere the key means something else:
// a text field (every letter is input), or a role=slider (the volume/seek
// handles own the arrow keys, and taking them away would break keyboard
// seeking). Reads the contenteditable ATTRIBUTE rather than isContentEditable,
// which jsdom does not implement.
export function isTypingTarget(target: EventTarget | null): boolean {
    if (!(target instanceof Element)) return false
    if (target.closest('input, textarea, select, [role="slider"]')) return true
    const editable = target.closest('[contenteditable]')
    return !!editable && editable.getAttribute('contenteditable') !== 'false'
}
