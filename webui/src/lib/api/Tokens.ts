import { apiClient } from '@/lib/api/client'
import { getDeviceId, getDeviceName } from '@/lib/deviceIdentity'
import type {
    ApiToken,
    CreateTokenInput,
    CreateTokenResponse,
    ListTokensResponse,
    MintSpaTokenResponse
} from '@/types/tokens'

/**
 * POST /api/v1/auth/token — mints the SPA's short-lived (48h) spa-scoped
 * token. Session-authorized: the cookie rides along; a 401 means the session
 * itself is gone.
 *
 * The device identity scopes the mint to THIS app instance: the server
 * supersedes only the session bearing the same deviceId, so the same user stays
 * signed in from several first-party apps, each listed under its deviceName.
 */
export async function mintSpaToken(): Promise<MintSpaTokenResponse> {
    const { data } = await apiClient.post<MintSpaTokenResponse>('/auth/token', {
        deviceId: getDeviceId(),
        deviceName: getDeviceName()
    })
    return data
}

/**
 * GET /api/v1/auth/tokens — all the caller's tokens: user-created PATs
 * (kind 'client') and live first-party SPA tokens (kind 'session').
 */
export async function listTokens(): Promise<ApiToken[]> {
    const { data } = await apiClient.get<ListTokensResponse>('/auth/tokens')
    return data.tokens ?? []
}

/** POST /api/v1/auth/tokens — the response is the only time the plaintext exists. */
export async function createToken(input: CreateTokenInput): Promise<CreateTokenResponse> {
    const { data } = await apiClient.post<CreateTokenResponse>('/auth/tokens', input)
    return data
}

export async function revokeToken(tokenId: string): Promise<void> {
    await apiClient.delete(`/auth/tokens/${encodeURIComponent(tokenId)}`)
}
