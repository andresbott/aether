import { useConfirm } from 'primevue/useconfirm'
import { useAuth } from '@/composables/useAuth'
import { isDesktopApp, serverHost, serverUrl } from '@/lib/host'

// The group the Change server question is addressed to, and that the one dialog
// answering it (mounted by PlayerLayout) listens on. PrimeVue shows every
// ungrouped ConfirmDialog for every ungrouped question, and the views mount
// ungrouped dialogs of their own (PlaylistDetailView, RadioStationDetailView,
// UserSettingsView), so an ungrouped question would appear in each of them.
export const CHANGE_SERVER_GROUP = 'change-server'

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
     * the login view's server step. Leaving empties the queue on this device and
     * signs the user out, hence the question first — but the sign-out is a
     * request to the server, so one that cannot be reached keeps the session
     * (in its cookie jar too), and the question promises no more than that.
     * switchServer() never rejects, which is why accept has nothing to catch.
     * Declining changes nothing.
     */
    function confirmChangeServer(): void {
        const url = serverUrl.value
        // Only the desktop app, and only once it has a server, has one to leave.
        if (!isDesktopApp() || url === null) return
        confirm.require({
            group: CHANGE_SERVER_GROUP,
            header: 'Change server',
            message: `Disconnect from ${serverHost(url)}? The queue on this device is cleared, and you are signed out if the server can be reached.`,
            icon: 'ms-warning',
            acceptLabel: 'Change server',
            rejectLabel: 'Cancel',
            acceptClass: 'p-button-danger',
            accept: () => void switchServer()
        })
    }

    return { confirmChangeServer }
}
