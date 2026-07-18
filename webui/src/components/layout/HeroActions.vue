<script setup lang="ts">
import Button from 'primevue/button'

withDefaults(
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
        <Button
            v-if="canStar"
            class="hero-action-star"
            :icon="starred ? 'pi pi-star-fill' : 'pi pi-star'"
            text
            rounded
            v-tooltip.bottom="'Toggle star'"
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
}
</style>
