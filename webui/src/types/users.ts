export type AuthMethod = 'none' | 'native'

/** Identity of the authenticated user; null until sessions exist. */
export interface MeUser {
    login: string
}

/** Server capabilities the SPA gates UI on (server config, not build-time). */
export interface ServerFeatures {
    userManagement: boolean
}

/** GET /api/v1/me — the SPA's bootstrap: auth method, identity, features. */
export interface MeResponse {
    authMethod: AuthMethod
    user: MeUser | null
    features: ServerFeatures
}

/**
 * The two user verticals: admins are members of the server's admin group,
 * regular users have no group membership.
 */
export type UserRole = 'admin' | 'user'

export interface User {
    /** Stable identity (UUID); updates and deletes are addressed by it. */
    id: string
    /** Mutable login name. */
    login: string
    enabled: boolean
    role: UserRole
}

export interface CreateUserInput {
    login: string
    password: string
    enabled?: boolean
    /** Defaults to 'user' on the server when omitted. */
    role?: UserRole
}

/** Partial update: omitted/empty fields are left untouched. */
export interface UpdateUserInput {
    login?: string
    password?: string
    enabled?: boolean
    role?: UserRole
}

export interface ListUsersResponse {
    users: User[]
    total: number
}
