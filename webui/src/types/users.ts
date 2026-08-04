export type AuthMethod = 'none' | 'native'

export interface AuthInfo {
    method: AuthMethod
}

export interface User {
    /** Stable identity (UUID); updates and deletes are addressed by it. */
    id: string
    /** Mutable login name. */
    login: string
    enabled: boolean
}

export interface CreateUserInput {
    login: string
    password: string
    enabled?: boolean
}

/** Partial update: omitted/empty fields are left untouched. */
export interface UpdateUserInput {
    login?: string
    password?: string
    enabled?: boolean
}

export interface ListUsersResponse {
    users: User[]
    total: number
}
