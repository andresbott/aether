import { computed } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as UsersApi from '@/lib/api/Users'
import type { AuthInfo, User, CreateUserInput, UpdateUserInput } from '@/types/users'
import { apiErrorMessage } from '@/lib/apiError'

export const userQueryKeys = {
    all: ['users'] as const,
    authInfo: ['auth-info'] as const
}

// The auth method is server config: it cannot change without a restart, so
// cache it for the session.
export function useAuthInfo() {
    return useQuery<AuthInfo>({
        queryKey: userQueryKeys.authInfo,
        queryFn: () => UsersApi.getAuthInfo(),
        staleTime: Infinity
    })
}

/** True when the server manages native users (gates the Users settings UI). */
export function useNativeAuth() {
    const { data } = useAuthInfo()
    return computed(() => data.value?.method === 'native')
}

export function useUsers() {
    return useQuery<User[]>({
        queryKey: userQueryKeys.all,
        queryFn: () => UsersApi.listUsers(),
        staleTime: 30 * 1000
    })
}

export function useCreateUser() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (input: CreateUserInput) => UsersApi.createUser(input),
        onSuccess: (user) => {
            qc.invalidateQueries({ queryKey: userQueryKeys.all })
            toast.add({
                severity: 'success',
                summary: 'User created',
                detail: user.login,
                life: 3000
            })
        },
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to create user',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}

export function useUpdateUser() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: ({ id, input }: { id: string; input: UpdateUserInput }) =>
            UsersApi.updateUser(id, input),
        onSuccess: (user) => {
            qc.invalidateQueries({ queryKey: userQueryKeys.all })
            toast.add({
                severity: 'success',
                summary: 'User updated',
                detail: user.login,
                life: 3000
            })
        },
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to update user',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}

export function useDeleteUser() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (id: string) => UsersApi.deleteUser(id),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: userQueryKeys.all })
            toast.add({
                severity: 'info',
                summary: 'User deleted',
                life: 3000
            })
        },
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to delete user',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}
