<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Dropdown from 'primevue/dropdown'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import RadioButton from 'primevue/radiobutton'
import type {
    IdentifyCandidate,
    IdentifyPick,
    IdentifyTrackResult,
    Track
} from '@/types/metadata'

// A candidate above this score is considered a confident match and its track
// is pre-accepted when the dialog opens.
const ACCEPT_THRESHOLD = 0.85

const props = defineProps<{
    visible: boolean
    results: IdentifyTrackResult[]
    tracks: Track[]
    // The dialog opens before the request resolves so the user sees the work
    // start; while this is true the body is a progress note and there is nothing
    // to review yet.
    loading: boolean
    // The files this run was launched for, so the progress note can name a count
    // before any result exists. `tracks` is the whole folder listing here (it is
    // the current-title lookup), so it cannot answer that.
    pending: Track[]
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    (e: 'apply', picks: IdentifyPick[]): void
    // Cancel is not just "close": it aborts the in-flight identify request, so
    // the parent needs to hear it rather than only observing visible: false.
    (e: 'cancel'): void
}>()

// Per-track review state: whether the user accepts the match, which candidate
// is chosen, and which of its releases the album fields come from.
interface RowState {
    accepted: boolean
    candidateIndex: number
    releaseIndex: number
}

const rows = ref(new Map<string, RowState>())

watch(
    () => props.results,
    (results) => {
        const next = new Map<string, RowState>()
        for (const r of results) {
            const best = r.candidates[0]
            next.set(r.path, {
                accepted: !r.error && best !== undefined && best.score >= ACCEPT_THRESHOLD,
                candidateIndex: 0,
                releaseIndex: 0
            })
        }
        rows.value = next
    },
    { immediate: true }
)

const trackByPath = computed(() => {
    const map = new Map<string, Track>()
    for (const t of props.tracks) map.set(t.path, t)
    return map
})

function rowState(path: string): RowState {
    return rows.value.get(path) ?? { accepted: false, candidateIndex: 0, releaseIndex: 0 }
}

function currentTitle(path: string): string {
    const t = trackByPath.value.get(path)
    return t?.title || '(no title)'
}

function scorePct(score: number): string {
    return `${Math.round(score * 100)}%`
}

function candidateLabel(c: IdentifyCandidate): string {
    const artists = c.artists.map((a) => a.name).join(', ')
    return artists ? `${c.title} — ${artists}` : c.title
}

function releaseOptions(c: IdentifyCandidate) {
    return c.releases.map((r, i) => {
        let label = r.year > 0 ? `${r.album} (${r.year})` : r.album
        if (r.track_number > 0) label += ` · track ${r.track_number}`
        return { label, value: i }
    })
}

function onCandidateChange(path: string) {
    // A different candidate has its own release list; restart at the first.
    const state = rows.value.get(path)
    if (state) state.releaseIndex = 0
}

const acceptedCount = computed(
    () => [...rows.value.values()].filter((s) => s.accepted).length
)

function apply() {
    const picks: IdentifyPick[] = []
    for (const r of props.results) {
        const state = rows.value.get(r.path)
        if (!state?.accepted) continue
        const candidate = r.candidates[state.candidateIndex]
        if (!candidate) continue
        picks.push({
            path: r.path,
            candidate,
            release: candidate.releases[state.releaseIndex] ?? null
        })
    }
    emit('apply', picks)
}

// One exit path for Cancel, the header X and an Escape press: all three mean
// "stop", and while a request is in flight stopping must abort it, not leave it
// running invisibly against the user's rate limit.
function cancel() {
    emit('cancel')
    emit('update:visible', false)
}
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="(v) => !v && cancel()"
        header="Identify tracks"
        modal
        :style="{ width: '72rem', maxWidth: '95vw' }"
    >
        <div v-if="loading" class="identify-loading" data-test="identify-loading">
            <i class="pi pi-spin pi-spinner"></i>
            <div class="loading-text">
                <p class="loading-headline">
                    Identifying {{ pending.length }} track{{ pending.length === 1 ? '' : 's' }}…
                </p>
                <!-- Sets the expectation: one fpcalc run plus a rate-limited
                     AcoustID call per file, so several seconds per track is
                     normal rather than a hang. -->
                <small class="loading-note">
                    Fingerprinting each file and looking it up on AcoustID. This can take a while;
                    Cancel stops it.
                </small>
            </div>
        </div>

        <div v-else class="identify-list">
            <div
                v-for="result in results"
                :key="result.path"
                class="identify-row"
                :data-test="`identify-row-${result.path}`"
            >
                <div class="row-header">
                    <Checkbox
                        v-if="!result.error && result.candidates.length > 0"
                        :modelValue="rowState(result.path).accepted"
                        @update:modelValue="(v: boolean) => (rowState(result.path).accepted = v)"
                        :binary="true"
                        :inputId="`accept-${result.path}`"
                    />
                    <label :for="`accept-${result.path}`" class="row-title">
                        <span class="row-path">{{ result.path }}</span>
                        <span class="row-current">current: {{ currentTitle(result.path) }}</span>
                    </label>
                </div>

                <small v-if="result.error" class="row-error" data-test="identify-error">
                    {{ result.error }}
                </small>
                <small
                    v-else-if="result.candidates.length === 0"
                    class="row-nomatch"
                    data-test="identify-nomatch"
                >
                    No match found.
                </small>

                <div v-else class="candidates">
                    <div
                        v-for="(candidate, ci) in result.candidates"
                        :key="candidate.recording_mbid"
                        class="candidate"
                    >
                        <RadioButton
                            :modelValue="rowState(result.path).candidateIndex"
                            @update:modelValue="
                                (v: number) => {
                                    rowState(result.path).candidateIndex = v
                                    onCandidateChange(result.path)
                                }
                            "
                            :value="ci"
                            :inputId="`cand-${result.path}-${ci}`"
                        />
                        <label :for="`cand-${result.path}-${ci}`" class="candidate-label">
                            <span class="candidate-score">{{ scorePct(candidate.score) }}</span>
                            {{ candidateLabel(candidate) }}
                        </label>
                        <Dropdown
                            v-if="
                                ci === rowState(result.path).candidateIndex &&
                                candidate.releases.length > 1
                            "
                            class="release-select"
                            :modelValue="rowState(result.path).releaseIndex"
                            @update:modelValue="
                                (v: number) => (rowState(result.path).releaseIndex = v)
                            "
                            :options="releaseOptions(candidate)"
                            optionLabel="label"
                            optionValue="value"
                        />
                        <span
                            v-else-if="
                                ci === rowState(result.path).candidateIndex &&
                                candidate.releases.length === 1
                            "
                            class="release-single"
                        >
                            {{ releaseOptions(candidate)[0].label }}
                        </span>
                    </div>
                </div>
            </div>
        </div>

        <template #footer>
            <Button label="Cancel" text data-test="identify-cancel" @click="cancel" />
            <Button
                v-if="!loading"
                :label="`Stage ${acceptedCount} track${acceptedCount === 1 ? '' : 's'}`"
                icon="pi pi-check"
                data-test="identify-apply"
                :disabled="acceptedCount === 0"
                @click="apply"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.identify-loading {
    display: flex;
    align-items: flex-start;
    gap: 0.9rem;
    padding: 2rem 0.5rem;
}
.identify-loading .pi-spinner {
    font-size: 1.6rem;
    color: var(--app-accent);
}
.loading-text {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    min-width: 0;
}
.loading-headline {
    margin: 0;
    font-weight: 600;
}
.loading-note {
    color: var(--app-text-secondary);
}
.identify-list {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    max-height: 75vh;
    overflow-y: auto;
}
.identify-row {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    padding: 0.6rem;
    border: 1px solid var(--app-border);
    border-radius: 6px;
}
.row-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}
.row-title {
    display: flex;
    flex-direction: column;
    min-width: 0;
    cursor: pointer;
}
.row-path {
    font-size: 0.85rem;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.row-current {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}
.row-error {
    color: var(--p-red-600, #dc2626);
}
.row-nomatch {
    color: var(--app-text-secondary);
}
.candidates {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    padding-left: 1.6rem;
}
.candidate {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 0;
}
.candidate-label {
    font-size: 0.85rem;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    cursor: pointer;
}
.candidate-score {
    display: inline-block;
    min-width: 2.6rem;
    font-variant-numeric: tabular-nums;
    color: var(--app-accent);
    font-weight: 600;
}
.release-select {
    margin-left: auto;
    max-width: 16rem;
}
.release-single {
    margin-left: auto;
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 16rem;
}
</style>
