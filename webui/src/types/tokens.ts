/** A user-created PAT as the management endpoints report it (never the hash). */
export interface ApiToken {
    tokenId: string
    name: string
    createdAt: string
    lastUsedAt?: string | null
    expiresAt?: string | null
}

/** POST /api/v1/auth/token — the SPA's short-lived spa-scoped token. */
export interface MintSpaTokenResponse {
    token: string
    tokenId: string
    expiresAt: string
}

export interface CreateTokenInput {
    name: string
    /** RFC3339; omitted = never expires. */
    expiresAt?: string
}

/** POST /api/v1/auth/tokens — the plaintext appears here and nowhere else. */
export interface CreateTokenResponse extends ApiToken {
    token: string
}

export interface ListTokensResponse {
    tokens: ApiToken[]
}
