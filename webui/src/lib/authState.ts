// Session-expiry signal shared between the axios layer and the auth
// composable. It lives outside pinia/vue-query so the response interceptor in
// client.ts can flip it without needing an app instance; useAuth folds it
// into the "needs login" decision and clears it after a successful login.
import { computed, ref } from 'vue'

export const sessionExpired = ref(false)

// True from the moment the user asks to log out until the next successful
// login. A logout ends the session too, and its device purge resets the query
// cache — those refetches 401 and flip `sessionExpired` just like a real
// expiry would, so the flag alone cannot tell the two apart. useAuth sets this
// when the logout starts.
export const explicitLogout = ref(false)

/**
 * The session ended without the user asking for it (expired, or the server
 * restarted with fresh cookie keys) — the login view explains it. A deliberate
 * logout says nothing: the user knows why they are looking at the form.
 */
export const sessionLostUnexpectedly = computed(
    () => sessionExpired.value && !explicitLogout.value
)
