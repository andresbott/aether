import { computed, ref } from 'vue'
import { useInfiniteQuery } from '@tanstack/vue-query'
import { subsonicClient } from '@/lib/api/subsonic'
import { queryKeys } from '@/composables/useSubsonicQueries'
import type { DiscoveryPage, DiscoveryFeedEntry } from '@/types/subsonic'

export const DISCOVERY_PAGE_SIZE = 48

// Pure: flattens the server's two per-type arrays into one feed ordered by the
// server-assigned rank. The ranking is authoritative and cross-type, so merging
// is a sort — the client never re-scores or re-weights anything.
//
// Deduped by id, keeping the lowest rank. The server pins the candidate pool so
// pages cannot drift on their own, but scores are still computed against live
// data: starring an album mid-scroll raises its favorite term and can move it into
// a range an earlier page already showed, where it would otherwise appear twice.
// No stateless design prevents that, so the client absorbs it — a silent skip
// beats a visible duplicate.
export function flattenDiscoveryPages(pages: DiscoveryPage[]): DiscoveryFeedEntry[] {
    const entries: DiscoveryFeedEntry[] = []
    for (const page of pages) {
        for (const al of page.album) {
            entries.push({ type: 'album', rank: al.rank, reason: al.reason, album: al })
        }
        for (const pl of page.playlist) {
            entries.push({ type: 'playlist', rank: pl.rank, reason: pl.reason, playlist: pl })
        }
    }
    entries.sort((a, b) => a.rank - b.rank)
    const seen = new Set<string>()
    return entries.filter((e) => {
        const id = e.type === 'album' ? e.album.id : e.playlist.id
        if (seen.has(id)) return false
        seen.add(id)
        return true
    })
}

// The terminal condition for the infinite feed, extracted so it is directly
// testable: a page that comes back SHORT means the ranking is exhausted. Without
// this check the feed would page forever against a library smaller than one page,
// and (because the server caps the pool) past the end every request would return
// an empty page that the client kept asking for again.
//
// Returns the next offset, or undefined to stop.
export function nextDiscoveryOffset(
    lastPage: DiscoveryPage,
    allPages: DiscoveryPage[]
): number | undefined {
    const got = lastPage.album.length + lastPage.playlist.length
    if (got < DISCOVERY_PAGE_SIZE) return undefined
    return allPages.length * DISCOVERY_PAGE_SIZE
}

export const DISCOVERY_SEED_WINDOW_MS = 12 * 60 * 60 * 1000

// Refresh has to change the seed or the query key would not change and nothing
// would reload. Multiplying the window out leaves room for reshuffles underneath
// it, so a refreshed feed is never accidentally the feed some future window would
// have produced on its own.
const DISCOVERY_RESHUFFLE_STRIDE = 1000

// Which 12-hour window a moment falls in — the feed's seed, so the ranking holds
// still for twelve hours instead of reshuffling on every visit.
//
// Pure and exported so the windowing is testable without mounting a query client
// or faking timers.
export function discoverySeedForTime(nowMs: number): number {
    return Math.floor(nowMs / DISCOVERY_SEED_WINDOW_MS)
}

// The ranked Discovery feed. The seed is part of the query key, so Refresh is a
// cache miss rather than a manual invalidation — and every page of one visit
// shares a seed, which is what keeps the sequence gap-free.
//
// The window is captured once per mount rather than read reactively: a feed the
// user is currently scrolling must not reshuffle underneath them just because a
// 12-hour boundary passed mid-session. The next visit picks up the new window.
export function useDiscoveryFeed() {
    const windowSeed = discoverySeedForTime(Date.now())
    const reshuffles = ref(0)
    const seed = computed(
        () => windowSeed * DISCOVERY_RESHUFFLE_STRIDE + reshuffles.value
    )

    const query = useInfiniteQuery({
        queryKey: computed(() => queryKeys.discovery(seed.value)),
        queryFn: ({ pageParam }) =>
            subsonicClient.getDiscovery(DISCOVERY_PAGE_SIZE, pageParam as number, seed.value),
        initialPageParam: 0,
        getNextPageParam: nextDiscoveryOffset,
        // Matches the seed window: within one window the server would return the
        // same ranking anyway, so refetching earlier only costs requests. Refresh
        // still reloads immediately, because it changes the key rather than
        // invalidating this entry.
        staleTime: DISCOVERY_SEED_WINDOW_MS
    })

    return {
        items: computed(() => flattenDiscoveryPages(query.data.value?.pages ?? [])),
        isLoading: computed(() => query.isLoading.value),
        isError: computed(() => query.isError.value),
        hasNextPage: computed(() => !!query.hasNextPage.value),
        isFetchingNextPage: computed(() => query.isFetchingNextPage.value),
        fetchNextPage: () => {
            if (query.hasNextPage.value && !query.isFetchingNextPage.value) {
                void query.fetchNextPage()
            }
        },
        // Advances the reshuffle counter rather than re-reading the clock: within a
        // 12-hour window the time-derived part is constant, so a clock-based seed
        // would not change the query key and Refresh would do nothing.
        refresh: () => {
            reshuffles.value += 1
        }
    }
}
