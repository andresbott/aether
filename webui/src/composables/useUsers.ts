import { computed } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as UsersApi from '@/lib/api/Users'
import type { MeResponse, User, CreateUserInput, UpdateUserInput } from '@/types/users'
import { apiErrorMessage, apiFieldErrors } from '@/lib/apiError'
import { isDesktopApp, serverUrl } from '@/lib/host'

export const userQueryKeys = {
    all: ['users'] as const,
    me: ['me'] as const
}

// Auth method and features are server config: they cannot change without a
// restart, so cache the bootstrap for the session.
export function useMe() {
    return useQuery<MeResponse>({
        queryKey: userQueryKeys.me,
        queryFn: () => UsersApi.getMe(),
        staleTime: Infinity,
        // Inside the desktop app there is no one to ask until the login view's
        // server step has picked a server. A disabled query is not loading, so
        // the gate renders that step instead of waiting.
        enabled: computed(() => !isDesktopApp() || serverUrl.value !== null),
        // A failed /me is asked again only on purpose (the server step's
        // Connect). The login view mounting on that failure would otherwise
        // refetch it, which clears the error, unmounts the view, fails again and
        // mounts it again, round after round. (In the browser nothing mounts on
        // a failed /me, so nothing changes there.)
        retryOnMount: false
    })
}

/** True when the server exposes the users CRUD (gates the Users settings UI). */
export function useUserManagement() {
    const { data } = useMe()
    return computed(() => data.value?.features.userManagement === true)
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
            // A 422 names the bad field in errors[]; the dialog shows it inline,
            // so toasting would double-report the same message.
            if (apiFieldErrors(err).length > 0) return
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
            // A 422 names the bad field in errors[]; the dialog shows it inline,
            // so toasting would double-report the same message.
            if (apiFieldErrors(err).length > 0) return
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
