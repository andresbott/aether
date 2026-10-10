import { apiClient } from '@/lib/api/client'
import type {
    MeResponse,
    User,
    CreateUserInput,
    UpdateUserInput,
    ListUsersResponse
} from '@/types/users'

// The SPA's bootstrap, so it is bounded: a server that silently drops packets
// would otherwise hold the window blank until the system gave up on the
// connection, minutes later, and again for every retry. A healthy server
// answers in milliseconds.
const ME_TIMEOUT_MS = 10_000

export async function getMe(): Promise<MeResponse> {
    const { data } = await apiClient.get<MeResponse>('/me', { timeout: ME_TIMEOUT_MS })
    return data
}

export async function listUsers(): Promise<User[]> {
    const { data } = await apiClient.get<ListUsersResponse>('/users')
    return data.users ?? []
}

export async function createUser(input: CreateUserInput): Promise<User> {
    const { data } = await apiClient.post<User>('/users', input)
    return data
}

export async function updateUser(id: string, input: UpdateUserInput): Promise<User> {
    const { data } = await apiClient.put<User>(`/users/${encodeURIComponent(id)}`, input)
    return data
}

export async function deleteUser(id: string): Promise<void> {
    await apiClient.delete(`/users/${encodeURIComponent(id)}`)
}
