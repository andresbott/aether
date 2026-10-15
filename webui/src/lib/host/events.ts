// What the player's Rust side tells the SPA, as events. Like the rest of the host
// layer it keeps @tauri-apps/api to itself, and in a browser tab, where there is
// no Rust side to hear from, it does nothing.
import { isTauri } from '@tauri-apps/api/core'

/** A command from the tray menu: Play / Pause or Next. */
export type TrayCommand = 'play_pause' | 'next'

// Pinned to events.rs (TRAY_COMMAND_EVENT, TRAY_PLAY_PAUSE, TRAY_NEXT): nothing
// links the two sides at build time.
const TRAY_COMMAND_EVENT = 'tray://command'

const isTrayCommand = (payload: unknown): payload is TrayCommand =>
    payload === 'play_pause' || payload === 'next'

/**
 * Calls `cb` for each command the tray menu sends. The tray lives in Rust but the
 * SPA's own player is the one playing, so the menu asks the webview to act.
 * Resolves once subscribed, to the function that unsubscribes. A payload that is
 * not a known command is ignored. In a browser tab there is no tray: resolves at
 * once to a no-op. Rejects if the app refuses the subscription.
 */
export async function onTrayCommand(cb: (cmd: TrayCommand) => void): Promise<() => void> {
    // Asks isTauri, not index.ts's isDesktopApp: index re-exports this file, and
    // the two importing each other would be a cycle.
    if (!isTauri()) return () => {}
    // Imported here, not at the top, so the browser never evaluates the module.
    const { listen } = await import('@tauri-apps/api/event')
    return listen<string>(TRAY_COMMAND_EVENT, (event) => {
        if (isTrayCommand(event.payload)) cb(event.payload)
    })
}
