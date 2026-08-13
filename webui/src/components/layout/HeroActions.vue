<script setup lang="ts">
import { computed } from 'vue'
import Button from 'primevue/button'

// The props keep the Subsonic wire vocabulary (`starred`, `star`); the visible
// wording is "favorite" to match the heart icon used everywhere.
const props = withDefaults(
    defineProps<{
        playLabel?: string
        playDisabled?: boolean
        busy?: boolean
        canQueue?: boolean
        canStar?: boolean
        starred?: boolean
    }>(),
    {
        playLabel: 'Play',
        playDisabled: false,
        busy: false,
        canQueue: false,
        canStar: false,
        starred: false
    }
)

const favoriteLabel = computed(() =>
    props.starred ? 'Remove from favorites' : 'Add to favorites'
)

const emit = defineEmits<{
    (e: 'play'): void
    (e: 'queue'): void
    (e: 'star'): void
}>()
</script>

<template>
    <div class="hero-actions">
        <Button
            class="hero-action-play"
            :label="playLabel"
            icon="pi pi-play"
            :disabled="playDisabled"
            :loading="busy"
            @click="emit('play')"
        />
        <Button
            v-if="canQueue"
            class="hero-action-queue"
            label="Add to queue"
            icon="pi pi-plus"
            severity="secondary"
            text
            @click="emit('queue')"
        />
        <!-- `secondary` like the queue action beside it: a favorite reads as
             favorite by the FILLED icon alone, not by colour — without a severity
             this rendered in the primary accent in BOTH states, which said nothing
             about the state and competed with the accent's real jobs (what is
             playing, what is actionable). See unified-play-experience.md. -->
        <Button
            v-if="canStar"
            class="hero-action-star"
            :icon="starred ? 'pi pi-heart-fill' : 'pi pi-heart'"
            :aria-label="favoriteLabel"
            severity="secondary"
            text
            rounded
            v-tooltip.bottom="favoriteLabel"
            @click="emit('star')"
        />
    </div>
</template>

<style scoped>
.hero-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.4rem;
    flex-wrap: wrap;
    justify-content: center;
}

/* A wrapped actions ROW is fine; a wrapped label ("Add\nto\nqueue") is not. */
.hero-actions :deep(.p-button-label) {
    white-space: nowrap;
}
</style>
