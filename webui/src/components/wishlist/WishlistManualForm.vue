<script setup lang="ts">
import { computed, ref } from 'vue'
import SelectButton from 'primevue/selectbutton'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import Button from 'primevue/button'
import { WISHLIST_KINDS } from '@/lib/wishlist'
import type { WishlistKind } from '@/types/wishlist'

/**
 * Adds a wishlist entry by hand, for music MusicBrainz does not know (a
 * bootleg, a rip of a radio show): a type, a title and an optional
 * description. The entry has no MusicBrainz id, so no scan can mark it found.
 * The host owns the request; this only collects and emits.
 *
 * It sits under the MusicBrainz search in `WishlistAddView` and is built in its
 * image, centred on the same axis: a pill input with a round button beside it
 * (the search row — the pill matches `SearchField`'s metrics), the kind as the
 * same icon-and-label button row as the search filters, then the description.
 * No visible labels, as in the search; each control carries its own name.
 */

// Mirrors the server's limits (handlers/wishlist).
const TITLE_MAX = 500
const DESCRIPTION_MAX = 2000

defineProps<{ saving?: boolean }>()
const emit = defineEmits<{
    (e: 'add', entry: { kind: WishlistKind; title: string; note: string }): void
}>()

const kindOptions = WISHLIST_KINDS.map((k) => ({ label: k.label, value: k.value, icon: k.icon }))
const kind = ref<WishlistKind>('album')
const title = ref('')
const description = ref('')

const valid = computed(() => title.value.trim().length > 0)

function submit() {
    if (!valid.value) return
    emit('add', { kind: kind.value, title: title.value.trim(), note: description.value.trim() })
}
</script>

<template>
    <form class="manual-form" aria-labelledby="manual-heading" @submit.prevent="submit">
        <div class="manual-divider">
            <h2 id="manual-heading" class="manual-heading">Can't find it? Add it by hand</h2>
        </div>
        <p class="manual-hint">
            For music MusicBrainz doesn't list. Without a MusicBrainz id a scan can't mark it found — remove
            it yourself once you have it.
        </p>

        <div class="manual-row">
            <InputText
                v-model="title"
                class="manual-title"
                :maxlength="TITLE_MAX"
                placeholder="Title — e.g. Live at the Paramount (bootleg)"
                aria-label="Title"
            />
            <Button
                type="submit"
                class="manual-add"
                label="Add"
                icon="ms-add"
                rounded
                :disabled="!valid"
                :loading="saving"
            />
        </div>

        <SelectButton
            v-model="kind"
            :options="kindOptions"
            optionLabel="label"
            optionValue="value"
            :allowEmpty="false"
            dataKey="value"
            class="as-button-group manual-kind"
            aria-label="Type"
        >
            <template #option="slotProps">
                <i :class="slotProps.option.icon"></i>
                <span>{{ slotProps.option.label }}</span>
            </template>
        </SelectButton>

        <Textarea
            v-model="description"
            class="manual-description"
            rows="2"
            auto-resize
            :maxlength="DESCRIPTION_MAX"
            placeholder="Description (optional) — which edition, where to find it…"
            aria-label="Description"
        />
    </form>
</template>

<style scoped>
/* The search hero's column: centred, the search row's width, its gap. */
.manual-form {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.9rem;
    width: 100%;
    max-width: 760px;
    margin-inline: auto;
}

/* A hairline either side of the heading, separating it from the results. */
.manual-divider {
    display: flex;
    align-items: center;
    gap: 1rem;
    width: 100%;
}
.manual-divider::before,
.manual-divider::after {
    content: '';
    flex: 1;
    border-top: 1px solid var(--app-border);
}
.manual-heading {
    margin: 0;
    font-size: 0.85rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
    white-space: nowrap;
}
.manual-hint {
    margin: -0.3rem 0 0.2rem;
    max-width: 34rem;
    text-align: center;
    font-size: 0.875rem;
    line-height: 1.45;
    color: var(--app-text-secondary);
}

/* The search row: a pill input with the round button beside it. */
.manual-row {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    width: 100%;
}
/* SearchField's pill, without the leading search icon's inset. */
.manual-title {
    flex: 1;
    min-width: 0;
    font-size: 1.1rem;
    padding: 0.85rem 1.5rem;
    border-radius: 9999px;
}
/* The search page's Search button. */
.manual-add {
    flex-shrink: 0;
    height: 3.1rem;
    padding-inline: 1.25rem;
}

.manual-kind {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
}

/* The pill's sibling: same type size and inset, corners as round as a box
   that grows can carry. */
.manual-description {
    width: 100%;
    font-size: 1rem;
    padding: 0.85rem 1.5rem;
    border-radius: 1.5rem;
    resize: none;
}

@media (max-width: 767.98px) {
    /* As the Search button: only its icon beside the box on a phone. */
    .manual-add :deep(.p-button-label) {
        display: none;
    }
    .manual-add {
        padding-inline: 0;
        width: 3.1rem;
    }
}
</style>
