import { computed, ref, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as WishlistApi from '@/lib/api/Wishlist'
import { apiErrorMessage, isCanceledError, isRateLimitError } from '@/lib/apiError'
import type { WishlistAddInput, WishlistItem, WishlistKind, WishlistSearchResult } from '@/types/wishlist'

export const wishlistQueryKeys = {
    all: ['wishlist'] as const,
    list: ['wishlist', 'list'] as const,
    detail: (id: number) => ['wishlist', id] as const
}

// The shortest search the server runs; mirrored so the view can say "keep
// typing" instead of sending a request it would refuse.
export const MIN_WISHLIST_QUERY_LENGTH = 2

// Found flags change when a scan finishes, not on a schedule, so a short stale
// time plus the refetch on mount is enough to show a scan's result.
const STALE_MS = 15 * 1000

// `enabled` lets a caller that only sometimes shows the wishlist (the phone's
// browse page, for admins) skip the request otherwise.
export function useWishlist(enabled: MaybeRefOrGetter<boolean> = true) {
    return useQuery<WishlistItem[]>({
        queryKey: wishlistQueryKeys.list,
        queryFn: () => WishlistApi.listWishlist(),
        staleTime: STALE_MS,
        enabled: computed(() => toValue(enabled))
    })
}

export function useWishlistItem(id: MaybeRefOrGetter<number>) {
    const qc = useQueryClient()
    return useQuery<WishlistItem>({
        queryKey: computed(() => wishlistQueryKeys.detail(toValue(id))),
        queryFn: () => WishlistApi.getWishlistItem(toValue(id)),
        enabled: computed(() => Number.isInteger(toValue(id)) && toValue(id) > 0),
        staleTime: STALE_MS,
        // Opening an entry from the list shows it at once.
        placeholderData: () =>
            qc.getQueryData<WishlistItem[]>(wishlistQueryKeys.list)?.find((it) => it.id === toValue(id))
    })
}

export function useAddWishlistItems() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (items: WishlistAddInput[]) => WishlistApi.addWishlistItems(items),
        onSuccess: (items) => {
            qc.invalidateQueries({ queryKey: wishlistQueryKeys.all })
            const found = items.filter((it) => it.in_library).length
            toast.add({
                severity: 'success',
                summary: items.length === 1 ? 'Added to the wishlist' : `${items.length} added to the wishlist`,
                detail:
                    found === 0
                        ? undefined
                        : found === 1 && items.length === 1
                          ? 'It is already in your library.'
                          : `${found} already in your library.`,
                life: 3000
            })
        },
        onError: (err: unknown) => {
            toast.add({
                severity: 'error',
                summary: 'Could not add to the wishlist',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}

export function useUpdateWishlistItem() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: ({ id, patch }: { id: number; patch: WishlistApi.WishlistUpdate }) =>
            WishlistApi.updateWishlistItem(id, patch),
        onSuccess: (item) => {
            qc.setQueryData(wishlistQueryKeys.detail(item.id), item)
            qc.invalidateQueries({ queryKey: wishlistQueryKeys.list })
            toast.add({ severity: 'success', summary: 'Saved', life: 3000 })
        },
        onError: (err: unknown) => {
            toast.add({
                severity: 'error',
                summary: 'Could not save the entry',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}

export function useDeleteWishlistItem() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (id: number) => WishlistApi.deleteWishlistItem(id),
        // Only the list is refreshed: dropping the detail query here would make
        // the still-mounted entry view refetch the deleted id before it leaves.
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: wishlistQueryKeys.list })
            toast.add({ severity: 'info', summary: 'Removed from the wishlist', life: 3000 })
        },
        onError: (err: unknown) => {
            toast.add({
                severity: 'error',
                summary: 'Could not remove the entry',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}

export type WishlistResultsByKind = Partial<Record<WishlistKind, WishlistSearchResult[]>>

// The MusicBrainz search behind the add view. It runs only when asked (the view
// calls it on Enter, its Search button or a filter click — never per
// keystroke), and caches each kind's answer for the submitted text: narrowing
// "All" to Albums costs no request, and switching to a kind not fetched yet
// costs one. A new text drops everything and aborts what is in flight, so a slow
// answer to an old text never lands — MusicBrainz is throttled to one request
// per second and answers out of order under load. A kind that failed is not
// cached, so searching again retries it.
export function useWishlistSearch() {
    // The text the results belong to; empty until a search is submitted.
    const term = ref('')
    const results = ref<WishlistResultsByKind>({})
    const pending = ref<WishlistKind[]>([])
    const error = ref<string | null>(null)
    const rateLimited = ref(false)
    const loading = computed(() => pending.value.length > 0)
    let generation = 0
    let controller: AbortController | null = null

    function clear() {
        controller?.abort()
        controller = null
        generation++
        term.value = ''
        results.value = {}
        pending.value = []
        error.value = null
        rateLimited.value = false
    }

    async function search(text: string, kinds: readonly WishlistKind[]) {
        const q = text.trim()
        if (q.length < MIN_WISHLIST_QUERY_LENGTH) return
        if (q !== term.value) {
            clear()
            term.value = q
        } else {
            error.value = null
            rateLimited.value = false
        }
        const missing = kinds.filter((k) => !(k in results.value) && !pending.value.includes(k))
        if (missing.length === 0) return

        controller ??= new AbortController()
        const { signal } = controller
        const mine = generation
        pending.value = [...pending.value, ...missing]
        await Promise.all(
            missing.map(async (kind) => {
                try {
                    const res = await WishlistApi.searchWishlist({ kind, q }, signal)
                    if (mine === generation) results.value = { ...results.value, [kind]: res }
                } catch (err: unknown) {
                    if (mine !== generation || isCanceledError(err)) return
                    error.value = apiErrorMessage(
                        err,
                        'The MusicBrainz search could not be completed. Try again in a moment.'
                    )
                    rateLimited.value = rateLimited.value || isRateLimitError(err)
                } finally {
                    if (mine === generation) pending.value = pending.value.filter((k) => k !== kind)
                }
            })
        )
    }

    return { term, results, pending, loading, error, rateLimited, search, clear }
}
