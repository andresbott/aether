<script setup lang="ts">
import { computed } from 'vue'
import type { AlbumLetter } from '@/types/subsonic'

const props = defineProps<{ letters: AlbumLetter[] }>()
const emit = defineEmits<{ (e: 'select', offset: number): void }>()

const RAIL = ['#', ...'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')]

const offsetByLetter = computed(() => {
    const m = new Map<string, number>()
    for (const l of props.letters) m.set(l.name, l.offset)
    return m
})

function onSelect(letter: string): void {
    const offset = offsetByLetter.value.get(letter)
    if (offset !== undefined) emit('select', offset)
}
</script>

<template>
    <div class="alphabet-rail">
        <button
            v-for="letter in RAIL"
            :key="letter"
            type="button"
            class="rail-letter"
            :disabled="!offsetByLetter.has(letter)"
            @click="onSelect(letter)"
        >
            {{ letter }}
        </button>
    </div>
</template>

<style scoped>
.alphabet-rail {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.05rem;
    position: sticky;
    top: 0;
    align-self: flex-start;
}

.rail-letter {
    background: none;
    border: none;
    padding: 0 0.25rem;
    font-size: 0.7rem;
    line-height: 1.2;
    cursor: pointer;
    color: var(--app-text-secondary);
    border-radius: 3px;
}

.rail-letter:hover:not(:disabled) {
    color: var(--p-primary-color);
}

.rail-letter:disabled {
    opacity: 0.3;
    cursor: default;
}
</style>
