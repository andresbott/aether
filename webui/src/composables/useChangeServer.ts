import { useConfirm } from 'primevue/useconfirm'
import { useAuth } from '@/composables/useAuth'
import { isDesktopApp, serverUrl } from '@/lib/host'

// The group the Change server question is addressed to, and that the one dialog
// answering it (mounted by PlayerLayout) listens on. PrimeVue shows every
// ungrouped ConfirmDialog for every ungrouped question, and the views mount
// ungrouped dialogs of their own (PlaylistDetailView, RadioStationDetailView,
// UserSettingsView), so an ungrouped question would appear in each of them.
export const CHANGE_SERVER_GROUP = 'change-server'

// `host[:port]` of the saved address: how the user knows the server, minus the scheme.
function hostOf(url: string): string {
    try {
        return new URL(url).host
    } catch {
        // Only a hand-edited settings file gets here; show what it holds.
        return url
    }
}

/**
 * Change server, for the account menus: UserMenu in the desktop shell and
 * MobileBrowseView's ⋮ menu in the phone shell. Each only has to call
 * confirmChangeServer() from its entry (rendered in the desktop app only); the
 * question itself is shown by the dialog PlayerLayout mounts, which sits outside
 * the shell swap and so serves both.
 */
export function useChangeServer() {
    const confirm = useConfirm()
    const { switchServer } = useAuth()

    /**
     * Asks whether to leave the current server and, once accepted, switches to
     * the login view's server step. Leaving signs the user out and empties the
     * queue on this device, hence the question first. switchServer() never
     * rejects, which is why accept has nothing to catch. Declining changes nothing.
     */
    function confirmChangeServer(): void {
        const url = serverUrl.value
        // Only the desktop app, and only once it has a server, has one to leave.
        if (!isDesktopApp() || url === null) return
        confirm.require({
            group: CHANGE_SERVER_GROUP,
            header: 'Change server',
            message: `Disconnect from ${hostOf(url)}? You will be signed out.`,
            icon: 'ms-warning',
            acceptLabel: 'Change server',
            rejectLabel: 'Cancel',
            acceptClass: 'p-button-danger',
            accept: () => void switchServer()
        })
    }

    return { confirmChangeServer }
}
