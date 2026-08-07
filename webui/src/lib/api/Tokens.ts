import { apiClient } from '@/lib/api/client'
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
 */
export async function mintSpaToken(): Promise<MintSpaTokenResponse> {
    const { data } = await apiClient.post<MintSpaTokenResponse>('/auth/token')
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
