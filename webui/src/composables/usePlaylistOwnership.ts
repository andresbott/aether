import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useAuth } from '@/composables/useAuth'

/**
 * Whether the current user owns (may edit) a playlist with the given owner login.
 *
 * Auth "none" has no per-user identity and the lone visitor owns everything, so a
 * null identity is treated as owner — matching the backend's fixed "admin" owner
 * and its error-50 write guard. In native / proxy-header modes it compares the
 * playlist's owner to the authenticated login.
 */
export function useIsPlaylistOwner(owner: MaybeRefOrGetter<string | undefined>) {
    const { currentUser } = useAuth()
    return computed(() => {
        const me = currentUser.value
        if (!me) return true
        return toValue(owner) === me.login
    })
}
