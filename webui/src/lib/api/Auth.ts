import { apiClient } from '@/lib/api/client'
import type { LoginResponse } from '@/types/users'

/**
 * POST /api/v1/auth/login — verifies credentials and sets the session cookie.
 * sessionRenew is the "remember me" bit: it opts the session into rolling
 * expiry renewal (activity keeps it alive) instead of a fixed window.
 * Every credential failure is a uniform 401.
 */
export async function login(
    username: string,
    password: string,
    sessionRenew: boolean
): Promise<LoginResponse> {
    const { data } = await apiClient.post<LoginResponse>('/auth/login', {
        username,
        password,
        sessionRenew
    })
    return data
}

/**
 * POST /api/v1/auth/logout — clears the session cookie; when the SPA's spa
 * token id is passed the server best-effort revokes it too.
 */
export async function logout(spaTokenId?: string): Promise<void> {
    await apiClient.post('/auth/logout', spaTokenId ? { tokenId: spaTokenId } : undefined)
}

/**
 * PUT /api/v1/auth/password — changes the signed-in user's own password after
 * re-verifying the current one (native mode only). The server revokes every
 * session and re-issues this caller's cookie, so this device stays signed in
 * while other devices are signed out. Answers 204; a wrong current password is
 * 401, throttled attempts 429.
 */
export async function changePassword(
    currentPassword: string,
    newPassword: string
): Promise<void> {
    await apiClient.put('/auth/password', { currentPassword, newPassword })
}
