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

/** POST /api/v1/auth/logout — clears the session cookie. */
export async function logout(): Promise<void> {
    await apiClient.post('/auth/logout')
}
