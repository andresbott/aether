// Session-expiry signal shared between the axios layer and the auth
// composable. It lives outside pinia/vue-query so the response interceptor in
// client.ts can flip it without needing an app instance; useAuth folds it
// into the "needs login" decision and clears it after a successful login.
import { ref } from 'vue'

export const sessionExpired = ref(false)
