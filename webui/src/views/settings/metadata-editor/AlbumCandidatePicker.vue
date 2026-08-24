<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import type { AlbumOption, Track } from '@/types/metadata'
import { albumChangeCounts, summarizeAlbumChanges } from '@/lib/albumChanges'

const props = defineProps<{
    visible: boolean
    // Best-first, as ranked by the resolver; the table keeps that order.
    options: AlbumOption[]
    // The release currently chosen in the identify dialog: its row opens selected.
    selectedMbid: string
    // The selection being identified — the denominator for coverage and the
    // basis for each option's change summary.
    tracks: Track[]
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    // A confirmed choice: the release the user settled on. The identify dialog
    // repoints its whole review to this MBID.
    (e: 'select', mbid: string): void
}>()

// Local until confirmed: browsing the table must not restage the review behind
// the dialog. Only "Use this release" commits.
const picked = ref('')

function pick(mbid: string) {
    picked.value = mbid
}

// Open (or a change of the incoming pick while open) arms the current choice;
// a selectedMbid absent from the options falls back to the best match.
watch(
    () => [props.visible, props.selectedMbid, props.options] as const,
    () => {
        if (!props.visible) return
        const has = props.options.some((o) => o.release_mbid === props.selectedMbid)
        picked.value = has ? props.selectedMbid : (props.options[0]?.release_mbid ?? '')
    },
    { immediate: true }
)

// The release's tracklist size, or a note that it was never fetched. Mirrors the
// identify dialog's own album detail line so the two read the same.
function sizeText(o: AlbumOption): string {
    if (!o.enriched) return 'track list unavailable'
    const parts = [`${o.track_count} track${o.track_count === 1 ? '' : 's'}`]
    if (o.disc_count > 1) parts.push(`${o.disc_count} discs`)
    return parts.join(' · ')
}

// Everything the table renders per option, computed once: the raw AlbumOption
// plus its display strings, coverage over the selection, confidence, size and a
// summary of the tags it would rewrite.
const rows = computed(() =>
    props.options.map((o) => ({
        o,
        album: o.album,
        // `?? []` despite the type: a Go nil slice marshals to JSON null, and one
        // missing credit list must degrade a cell, not take the table down.
        artist: (o.artists ?? [])
            .map((a) => a.name)
            .filter((n) => n !== '')
            .join(', '),
        year: o.year > 0 ? String(o.year) : '',
        coverage: `${o.matched_count} / ${props.tracks.length}`,
        confidence: `${Math.round((o.mean_score ?? 0) * 100)}%`,
        size: sizeText(o),
        changes: summarizeAlbumChanges(albumChangeCounts(o, props.tracks)),
        mbUrl: `https://musicbrainz.org/release/${o.release_mbid}`
    }))
)

const canConfirm = computed(() => picked.value !== '')

function confirm() {
    if (!canConfirm.value) return
    emit('select', picked.value)
    emit('update:visible', false)
}

function cancel() {
    emit('update:visible', false)
}
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="(v: boolean) => !v && cancel()"
        header="Choose the matching release"
        modal
        :style="{ width: '88rem', maxWidth: '96vw' }"
        class="candidate-picker-dialog"
    >
        <p class="picker-intro">
            {{ options.length }} releases matched this selection. Pick the one these files belong
            to; the table shows how well each fits and what it would change.
        </p>

        <table class="candidate-table">
            <caption class="sr-only">
                Matching releases, best match first
            </caption>
            <thead>
                <tr>
                    <th scope="col" class="col-select"><span class="sr-only">Select</span></th>
                    <th scope="col">Album</th>
                    <th scope="col">Artist</th>
                    <th scope="col" class="col-num">Year</th>
                    <th scope="col" class="col-num">Match</th>
                    <th scope="col" class="col-num">Confidence</th>
                    <th scope="col">Size</th>
                    <th scope="col">Changes</th>
                </tr>
            </thead>
            <tbody>
                <tr
                    v-for="row in rows"
                    :key="row.o.release_mbid"
                    class="candidate-row"
                    :class="{ selected: row.o.release_mbid === picked }"
                    :data-test="`candidate-row-${row.o.release_mbid}`"
                    @click="pick(row.o.release_mbid)"
                >
                    <td class="col-select">
                        <input
                            type="radio"
                            name="album-candidate"
                            class="candidate-radio"
                            :data-test="`candidate-radio-${row.o.release_mbid}`"
                            :checked="row.o.release_mbid === picked"
                            :aria-label="`Choose ${row.album}`"
                            @change="pick(row.o.release_mbid)"
                        />
                    </td>
                    <td class="col-album">
                        <span class="album-name">{{ row.album }}</span>
                        <a
                            class="mb-link"
                            :data-test="`candidate-mb-${row.o.release_mbid}`"
                            :href="row.mbUrl"
                            target="_blank"
                            rel="noopener"
                            title="View release on MusicBrainz"
                            @click.stop
                        >
                            <i class="pi pi-external-link"></i>
                        </a>
                    </td>
                    <td class="col-artist">{{ row.artist }}</td>
                    <td class="col-num col-year">{{ row.year }}</td>
                    <td class="col-num">{{ row.coverage }}</td>
                    <td class="col-num">{{ row.confidence }}</td>
                    <td class="col-size" :class="{ unenriched: !row.o.enriched }">{{ row.size }}</td>
                    <td
                        class="col-changes"
                        :data-test="`candidate-changes-${row.o.release_mbid}`"
                    >
                        {{ row.changes }}
                    </td>
                </tr>
            </tbody>
        </table>

        <template #footer>
            <Button label="Cancel" text data-test="candidate-cancel" @click="cancel" />
            <Button
                label="Use this release"
                icon="pi pi-check"
                data-test="candidate-confirm"
                :disabled="!canConfirm"
                @click="confirm"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.picker-intro {
    margin: 0 0 1rem;
    color: var(--app-text-secondary);
}

/* A plain semantic table rather than a DataTable: the candidate set is small
   (a handful of releases), the columns are fixed, and a native table keeps the
   selection radios and the row highlight trivially controllable — matching the
   identify dialog's own hand-rolled track table. */
.candidate-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.9rem;
}
.candidate-table th {
    text-align: left;
    padding: 0.4rem 0.75rem;
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
    border-bottom: 1px solid var(--app-border);
}
.candidate-table td {
    padding: 0.55rem 0.75rem;
    border-bottom: 1px solid var(--app-border);
    vertical-align: middle;
}
.candidate-row {
    cursor: pointer;
}
.candidate-row:hover {
    background: var(--app-hover);
}
.candidate-row.selected {
    background: var(--app-accent-soft);
}
.col-num {
    text-align: right;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
}
.col-select {
    width: 2.2rem;
    text-align: center;
}
.candidate-radio {
    cursor: pointer;
}
.col-album {
    font-weight: 600;
}
.album-name {
    margin-right: 0.4rem;
}
.mb-link {
    color: var(--app-text-secondary);
    font-size: 0.8rem;
}
.mb-link:hover {
    color: var(--app-accent);
}
.col-artist {
    color: var(--app-text-secondary);
}
/* An un-enriched option has no tracklist, so its size note reads as absent
   information rather than a real count. */
.col-size.unenriched {
    color: var(--app-text-secondary);
    font-style: italic;
}
/* The tags a save would rewrite — the same emphasis the identify dialog gives a
   staged value, so the two surfaces read as one. */
.col-changes {
    color: var(--app-staged);
    font-weight: 600;
}

/* Visually hidden but reachable by screen readers — for the selection column's
   header and each radio's caption. */
.sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
}
</style>
