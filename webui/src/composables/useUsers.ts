import { computed } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as UsersApi from '@/lib/api/Users'
import type { MeResponse, MeUser, User, CreateUserInput, UpdateUserInput } from '@/types/users'
import { apiErrorMessage, apiFieldErrors } from '@/lib/apiError'
import { isDesktopApp, serverUrl, sessionKind } from '@/lib/host'
import { subsonicClient } from '@/lib/api/subsonic'
import { subsonicReady } from '@/lib/subsonicSession'

export const userQueryKeys = {
    all: ['users'] as const,
    me: ['me'] as const,
    // A key session's identity (getUser). Deliberately outside the ['subsonic']
    // prefix: a favorite toggle invalidates that whole prefix, and the identity
    // is the session's own, asked once — not again with every catalog refresh.
    restIdentity: ['restIdentity'] as const
}

// Auth method and features are server config: they cannot change without a
// restart, so cache the bootstrap for the session.
export function useMe() {
    return useQuery<MeResponse>({
        queryKey: userQueryKeys.me,
        queryFn: () => UsersApi.getMe(),
        staleTime: Infinity,
        // Inside the desktop app there is no one to ask until the login view's
        // server step has picked a server, and nothing to ask in a key session:
        // that session is /rest only, its server may keep /api/v0 behind a
        // portal, and its identity comes from useRestIdentity. A disabled query
        // is not loading, so the gate renders its card instead of waiting.
        enabled: computed(
            () => !isDesktopApp() || (serverUrl.value !== null && sessionKind.value === 'cookie')
        ),
        // A failed /me is asked again only on purpose (the server step's
        // Connect). The login view mounting on that failure would otherwise
        // refetch it, which clears the error, unmounts the view, fails again and
        // mounts it again, round after round. (In the browser nothing mounts on
        // a failed /me, so nothing changes there.)
        retryOnMount: false,
        // Nor, in the app, when the network comes back: the refetch would put
        // the errored /me back to pending and take the server step away
        // mid-typing. A tab opened offline recovers through it, so the browser
        // keeps it.
        refetchOnReconnect: !isDesktopApp(),
        // In the app Connect is the retry: three more attempts, each bounded by
        // getMe's 10 s timeout, would keep the window blank for most of a minute
        // before the server step could show. The browser keeps its retries.
        retry: isDesktopApp() ? false : 3
    })
}

/**
 * A key session's identity: the caller's own record over /rest (getUser), in
 * the shape /me gives (login and role). Asked once the key is installed on the
 * subsonic client (subsonicReady), never in a cookie session. The app's /me
 * rules apply — no retry (the server step's Connect is the retry), no refetch
 * on mount or reconnect — and the request itself is bounded in the client.
 */
export function useRestIdentity() {
    return useQuery<MeUser>({
        queryKey: userQueryKeys.restIdentity,
        queryFn: async () => {
            const user = await subsonicClient.getUser()
            return { login: user.username, role: user.adminRole ? 'admin' : 'user' }
        },
        staleTime: Infinity,
        enabled: computed(() => sessionKind.value === 'apiKey' && subsonicReady.value),
        retryOnMount: false,
        refetchOnReconnect: false,
        retry: false
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
