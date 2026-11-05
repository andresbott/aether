import { apiClient } from '@/lib/api/client'
import type { WishlistAddInput, WishlistItem, WishlistKind, WishlistSearchResult } from '@/types/wishlist'

export async function listWishlist(): Promise<WishlistItem[]> {
    const { data } = await apiClient.get<{ items: WishlistItem[] }>('/wishlist')
    return data.items ?? []
}

export async function getWishlistItem(id: number): Promise<WishlistItem> {
    const { data } = await apiClient.get<WishlistItem>(`/wishlist/${id}`)
    return data
}

// Adding is idempotent per (kind, mbid): an already-listed pick comes back as
// the stored entry. An entry added by hand (no mbid) is always new.
export async function addWishlistItems(items: WishlistAddInput[]): Promise<WishlistItem[]> {
    const { data } = await apiClient.post<{ items: WishlistItem[] }>('/wishlist', { items })
    return data.items ?? []
}

// A partial update: `note` on any entry; `title` and `kind` only on an entry
// added by hand (the server refuses them on a MusicBrainz pick).
export interface WishlistUpdate {
    note?: string
    title?: string
    kind?: WishlistKind
}

export async function updateWishlistItem(id: number, patch: WishlistUpdate): Promise<WishlistItem> {
    const { data } = await apiClient.put<WishlistItem>(`/wishlist/${id}`, patch)
    return data
}

export async function deleteWishlistItem(id: number): Promise<void> {
    await apiClient.delete(`/wishlist/${id}`)
}

export interface WishlistSearchParams {
    kind: WishlistKind
    // Free text: a title, a name, or both ("in utero nirvana").
    q: string
}

export async function searchWishlist(
    params: WishlistSearchParams,
    signal?: AbortSignal
): Promise<WishlistSearchResult[]> {
    const { data } = await apiClient.get<{ results: WishlistSearchResult[] }>('/wishlist/search', {
        params: { kind: params.kind, q: params.q },
        signal
    })
    return data.results ?? []
}
