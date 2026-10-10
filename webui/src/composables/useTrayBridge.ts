import { effectScope, type EffectScope } from 'vue'
import { usePlayer } from '@/composables/usePlayer'
import { isDesktopApp, onTrayCommand } from '@/lib/host'

// The desktop app's tray menu lives in Rust, but the player that plays is this
// SPA's: the menu's Play / Pause and Next arrive as events and act through the
// same player actions the media keys use (useMediaSession), so the tray and the
// keys cannot diverge. Bound once from App.vue and for good, not per layout: the
// player is a module-level singleton that keeps playing on the settings routes,
// where PlayerLayout is unmounted, so the tray has to keep working there too.
let installed = false
// Detached, like useMediaSession's: no component adopts it, so the unmount of
// whichever one called us cannot dispose what the bridge owns.
let scope: EffectScope | null = null

export function useTrayBridge(): void {
    if (installed || !isDesktopApp()) return
    installed = true
    scope = effectScope(true)
    // usePlayer() registers its watchers in whatever scope is active, so it runs
    // in this one rather than in the caller's.
    scope.run(() => {
        void subscribe(usePlayer())
    })
}

async function subscribe(player: ReturnType<typeof usePlayer>): Promise<void> {
    try {
        // Nothing ever ends the bridge, so the function that unsubscribes is not kept.
        await onTrayCommand((command) => {
            if (command === 'play_pause') player.togglePlayPause()
            else if (command === 'next') player.playNext()
        })
    } catch (err) {
        // Only the tray menu is lost; the app works without it.
        console.warn('Tray commands unavailable', err)
    }
}

/** Test hook: allow a fresh install with new mocks. */
export function resetTrayBridgeForTests(): void {
    scope?.stop()
    scope = null
    installed = false
}
