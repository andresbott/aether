/** 'session' = first-party token minted by the SPA; 'client' = user-created PAT. */
export type ApiTokenKind = 'session' | 'client'

/** 'apikey' = presented whole via apiKey=; 'usertoken' = virtual username + password for Subsonic apps. */
export type ApiTokenType = 'apikey' | 'usertoken'

/** A token as the management endpoints report it (never the hash). */
export interface ApiToken {
    tokenId: string
    name: string
    kind: ApiTokenKind
    type: ApiTokenType
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
    type: ApiTokenType
    /** RFC3339; omitted = never expires. */
    expiresAt?: string
}

/** POST /api/v1/auth/tokens — the plaintext appears here and nowhere else. */
export interface CreateTokenResponse extends ApiToken {
    token: string
    /** usertoken only: the virtual username (= tokenId) to enter in the app. */
    username?: string
    /** usertoken only: the password to enter in the app. */
    password?: string
}

export interface ListTokensResponse {
    tokens: ApiToken[]
}
