import { apiClient } from '@/lib/api/client'
import type {
    MeResponse,
    User,
    CreateUserInput,
    UpdateUserInput,
    ListUsersResponse
} from '@/types/users'

export async function getMe(): Promise<MeResponse> {
    const { data } = await apiClient.get<MeResponse>('/me')
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
