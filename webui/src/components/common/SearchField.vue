<script setup lang="ts">
import InputText from 'primevue/inputtext'
import Button from 'primevue/button'

/**
 * The pill-shaped search box: a leading search icon, the input, and a reset
 * button inside its right edge while there is something to reset. Shared by the
 * library search (`SearchView`, which searches as you type) and the wishlist's
 * MusicBrainz search (`WishlistAddView`, which searches on `submit` — Enter or
 * its own button — to spare the rate-limited API).
 */
withDefaults(
    defineProps<{
        modelValue: string
        placeholder?: string
        // Shows the reset button; the host decides what counts as resettable
        // (SearchView: a term OR a narrowed scope).
        resettable?: boolean
        resetLabel?: string
        autofocus?: boolean
    }>(),
    { placeholder: '', resettable: false, resetLabel: 'Reset search', autofocus: false }
)

const emit = defineEmits<{
    (e: 'update:modelValue', value: string): void
    (e: 'reset'): void
    (e: 'submit'): void
}>()
</script>

<template>
    <span class="search-input-wrapper">
        <i class="ms-search search-icon"></i>
        <InputText
            :model-value="modelValue"
            :placeholder="placeholder"
            class="search-input"
            :class="{ 'has-reset': resettable }"
            :autofocus="autofocus"
            @update:model-value="emit('update:modelValue', $event ?? '')"
            @keydown.enter.prevent="emit('submit')"
        />
        <!-- The positioning lives on this span, not on the Button: PrimeVue's
             ripple directive writes `position: relative` as an INLINE style,
             which no stylesheet rule can beat, so an absolutely-positioned
             Button silently lays out in the flex flow and juts past the
             input's right edge. -->
        <span v-if="resettable" class="reset-search-slot">
            <Button
                class="reset-search"
                icon="ms-close"
                text
                rounded
                v-tooltip.bottom="resetLabel"
                :aria-label="resetLabel"
                @click="emit('reset')"
            />
        </span>
    </span>
</template>

<style scoped>
.search-input-wrapper {
    position: relative;
    display: flex;
    align-items: center;
    width: 100%;
    max-width: 640px;
}

.search-icon {
    position: absolute;
    left: 1.25rem;
    font-size: 1.1rem;
    color: var(--app-text-secondary);
    pointer-events: none;
}

.search-input {
    width: 100%;
    font-size: 1.1rem;
    padding: 0.85rem 1.5rem 0.85rem 3.25rem;
    border-radius: 9999px;
}

/* Keep the term clear of the reset button, which only exists while it can act. */
.search-input.has-reset {
    padding-right: 3.25rem;
}

/* Overlay the button on the input's right edge (see the template comment). */
.reset-search-slot {
    position: absolute;
    right: 0.5rem;
    display: flex;
    align-items: center;
}

.reset-search {
    width: 2.25rem;
    height: 2.25rem;
    color: var(--app-text-secondary);
}
</style>
