import { ref } from 'vue'
import { useIdentifyAlbum, useIdentifyTracks } from '@/composables/useMetadataEditor'
import { mergeTrackResults, useIdentifyCache } from '@/composables/useIdentifyCache'
import type { AlbumOption, IdentifyTrackResult, Track } from '@/types/metadata'

// RunOptions.force skips the cache for this run: the user explicitly asked for a
// fresh answer (re-identify), which is the only way past a cached one.
export interface RunOptions {
    force?: boolean
}

/**
 * useIdentifyRuns drives the metadata editor's two identify flows — per-track
 * and per-album — over the shared identify cache. It owns the dialog visibility,
 * the results the dialogs render, and the abort controllers behind Cancel.
 *
 * The cache is what makes reopening a dialog free: a run whose answers are all
 * cached never touches the network, so `isIdentifying` stays false and the
 * dialog opens straight onto the results rather than flashing its progress note.
 */
export function useIdentifyRuns(libraryId: () => number | null) {
    const cache = useIdentifyCache()
    const identifyMutation = useIdentifyTracks()
    const identifyAlbumMutation = useIdentifyAlbum()

    // ----- Per-track identification -----

    const trackResults = ref<IdentifyTrackResult[]>([])
    const trackDialog = ref(false)
    // The files the current run was launched for, so the dialog's progress note
    // can name a count before any result exists.
    const pendingTracks = ref<Track[]>([])
    // Only a run that actually reaches the server is "identifying": a fully
    // cached run resolves before the dialog can render, and a spinner for it
    // would flicker over results that are already there. Tracked here rather
    // than read off the mutation's isPending, which cannot tell the difference.
    const isIdentifying = ref(false)

    // The controller for the request currently in flight, so Cancel can abort it.
    let identifyAbort: AbortController | null = null

    async function identify(tracks: Track[], opts: RunOptions = {}) {
        const lib = libraryId()
        if (lib === null || tracks.length === 0) return

        const paths = tracks.map((t) => t.path)
        const { cached, missing } = opts.force
            ? { cached: [], missing: paths }
            : cache.getTrackResults(lib, paths)

        // Open on click, before any request resolves: each uncached file costs a
        // fingerprint run plus a rate-limited AcoustID call, so a button that
        // looks inert that long reads as broken. Seeding the results with the
        // cached ones means a fully cached run opens straight onto the review
        // table with no progress state at all.
        pendingTracks.value = tracks
        trackResults.value = cached
        trackDialog.value = true
        if (missing.length === 0) return

        const abort = new AbortController()
        identifyAbort = abort
        isIdentifying.value = true
        try {
            const fresh = await identifyMutation.mutateAsync({
                body: { library_id: lib, paths: missing },
                signal: abort.signal
            })
            // A response that arrives after the user cancelled (or started
            // another run) must not repopulate a dialog they already dismissed —
            // nor land in the cache under this run's paths.
            if (identifyAbort !== abort) return
            cache.putTrackResults(lib, fresh)
            trackResults.value = mergeTrackResults(paths, cached, fresh)
        } catch {
            // The mutation toasts real failures and stays silent on a cancel;
            // either way there is nothing new to review. Cached answers are
            // still worth showing, so only a run with nothing at all closes.
            if (identifyAbort === abort && cached.length === 0) trackDialog.value = false
        } finally {
            if (identifyAbort === abort) {
                identifyAbort = null
                isIdentifying.value = false
            }
        }
    }

    // Cancel means stop the work, not just hide the dialog: the request is
    // holding a fingerprint pass and rate-limited AcoustID lookups open.
    function cancelIdentify() {
        identifyAbort?.abort()
        identifyAbort = null
        isIdentifying.value = false
    }

    // ----- Album identification -----

    const albumOptions = ref<AlbumOption[]>([])
    const albumPathErrors = ref<Array<{ path: string; error: string }>>([])
    const albumTracks = ref<Track[]>([])
    const albumDialog = ref(false)
    // Same rule as isIdentifying: a cache hit is not a load.
    const isIdentifyingAlbum = ref(false)

    let albumAbort: AbortController | null = null

    async function identifyAlbum(tracks: Track[], opts: RunOptions = {}) {
        const lib = libraryId()
        // Album identification maps a SET onto one release, so a lone file has
        // nothing to map; per-track Identify is strictly better there.
        if (lib === null || tracks.length < 2) return

        const paths = tracks.map((t) => t.path)
        const hit = opts.force ? undefined : cache.getAlbumResponse(lib, paths)

        albumTracks.value = tracks
        albumOptions.value = hit?.options ?? []
        albumPathErrors.value = hit?.errors ?? []
        albumDialog.value = true
        if (hit) return

        const abort = new AbortController()
        albumAbort = abort
        isIdentifyingAlbum.value = true
        try {
            const out = await identifyAlbumMutation.mutateAsync({
                body: { library_id: lib, paths },
                signal: abort.signal
            })
            if (albumAbort !== abort) return
            cache.putAlbumResponse(lib, paths, out)
            albumOptions.value = out.options
            albumPathErrors.value = out.errors
        } catch {
            // Nothing to review and nothing cached to fall back on, so close
            // rather than show an empty dialog the user has to dismiss.
            if (albumAbort === abort) albumDialog.value = false
        } finally {
            if (albumAbort === abort) {
                albumAbort = null
                isIdentifyingAlbum.value = false
            }
        }
    }

    function cancelAlbumIdentify() {
        albumAbort?.abort()
        albumAbort = null
        isIdentifyingAlbum.value = false
    }

    // forgetAll throws away every cached answer. Wired to the editor's Reload
    // button: reloading is the user saying "read this folder again", and a
    // cached fingerprint answer for a file that has since been replaced on disk
    // is exactly the stale case they are trying to clear.
    function forgetAll() {
        cache.clear()
    }

    return {
        trackResults,
        trackDialog,
        pendingTracks,
        isIdentifying,
        identify,
        cancelIdentify,
        albumOptions,
        albumPathErrors,
        albumTracks,
        albumDialog,
        isIdentifyingAlbum,
        identifyAlbum,
        cancelAlbumIdentify,
        forgetAll
    }
}
