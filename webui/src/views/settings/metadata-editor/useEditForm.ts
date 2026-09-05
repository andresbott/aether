import { computed, type ComputedRef, type WritableComputedRef } from 'vue'
import type { Track, TrackOverlay } from '@/types/metadata'
import { diffInitialValues, type FieldDiff, type InitialValues } from '@/composables/useMetadataEditor'
import type { EditSession } from '@/composables/useEditSession'

export type TextKey =
    | 'title' | 'album' | 'mb_recording_id' | 'mb_release_id' | 'mb_release_group_id' | 'disc_subtitle'
export type NumKey = 'year' | 'track_number' | 'disc_number'
type PlaceholderKey = TextKey | NumKey

// Per-recording fields are only editable on a single track; guarded so a mass
// selection can never stage them even if a set slips through.
const PER_RECORDING = new Set<TextKey | NumKey>(['title', 'mb_recording_id', 'track_number'])

// numBuffer maps a numeric field diff onto the nullable input model: a mixed
// selection and an unset tag (0) both show an empty box, so the placeholder is
// visible in either case.
function numBuffer(d: FieldDiff<number>): number | null {
    if (!d.shared) return null
    return d.value === 0 ? null : d.value
}

export function useEditForm(selection: () => Track[], session: EditSession) {
    const isMass = computed(() => selection().length > 1)
    const paths = computed(() => selection().map((t) => t.path))

    // The editor displays and diffs EFFECTIVE values: original tags plus staged
    // (unsaved) session edits. `originalDiff` keeps the untouched baseline for
    // the overwrite-vs-leave rule and the undo tooltips.
    const effective = computed(() => selection().map((t) => session.effective(t)))
    const diff = computed<InitialValues>(() => diffInitialValues(effective.value))
    const originalDiff = computed<InitialValues>(() => diffInitialValues(selection()))

    function text(key: TextKey): WritableComputedRef<string> {
        return computed({
            get: () => diff.value[key].value,
            set: (v) => {
                if (isMass.value && PER_RECORDING.has(key)) return
                session.stageField(paths.value, key, v)
            }
        })
    }
    function num(key: NumKey): WritableComputedRef<number | null> {
        return computed({
            get: () => numBuffer(diff.value[key]),
            set: (v) => {
                if (isMass.value && PER_RECORDING.has(key)) return
                session.stageField(paths.value, key, v === null ? 0 : v)
            }
        })
    }
    const compilation: WritableComputedRef<boolean> = computed({
        get: () => diff.value.compilation.value,
        set: (v) => session.stageField(paths.value, 'compilation', v)
    })
    const genres: WritableComputedRef<string[]> = computed({
        get: () => (diff.value.genres.shared ? [...diff.value.genres.value] : []),
        set: (list) => {
            const mixed = isMass.value && !originalDiff.value.genres.shared
            if (mixed && list.length === 0) session.unstageField(paths.value, 'genres')
            else session.stageField(paths.value, 'genres', [...list])
        }
    })

    function placeholder(key: PlaceholderKey): ComputedRef<string> {
        return computed(() => (diff.value[key].shared ? '' : '(multiple values)'))
    }

    const genresMixed = computed(() => isMass.value && !diff.value.genres.shared)
    const compilationMixed = computed(() => isMass.value && !diff.value.compilation.shared)

    const dirtyFields = computed<Set<keyof TrackOverlay>>(() => {
        const s = new Set<keyof TrackOverlay>()
        for (const p of paths.value) {
            const overlay = session.overlays.value.get(p)
            if (overlay) for (const k of Object.keys(overlay)) s.add(k as keyof TrackOverlay)
        }
        return s
    })
    const isDirty = (key: keyof TrackOverlay) => dirtyFields.value.has(key)

    // Reverting a field just unstages it; the computeds reflect the original
    // effective value automatically — no buffer to refresh.
    function undo(key: keyof TrackOverlay) {
        session.unstageField(paths.value, key)
    }

    function undoTooltip(key: keyof InitialValues): string {
        const d = originalDiff.value[key]
        if (!d.shared) return 'Revert to each track\'s own value'
        const v = d.value
        if (typeof v === 'boolean') return `Revert to ${v ? 'yes' : 'no'}`
        if (v === '' || v === 0) return 'Revert to empty'
        return `Revert to "${v}"`
    }
    function undoGenresTooltip(): string {
        const d = originalDiff.value.genres
        if (!d.shared) return 'Revert to each track\'s own value'
        if (d.value.length === 0) return 'Revert to empty'
        return `Revert to "${d.value.join(', ')}"`
    }

    return {
        isMass, paths, effective, diff, originalDiff,
        text, num, compilation, genres, placeholder,
        genresMixed, compilationMixed,
        dirtyFields, isDirty, undo, undoTooltip, undoGenresTooltip
    }
}
