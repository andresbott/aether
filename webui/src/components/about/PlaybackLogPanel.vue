<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import { clearPlaybackLog, formatPlaybackLog, formatTime, playbackLog } from '@/lib/playbackLog'

// The player's on-device log (lib/playbackLog.ts), newest first: the only
// record of why playback stopped on a locked phone. Copy hands the whole log,
// oldest first, to a bug report.
const newestFirst = computed(() => [...playbackLog.value].reverse())

const copied = ref(false)
let copiedTimer: ReturnType<typeof setTimeout> | null = null

async function copyLog(): Promise<void> {
    try {
        await navigator.clipboard.writeText(formatPlaybackLog(playbackLog.value))
        copied.value = true
        if (copiedTimer) clearTimeout(copiedTimer)
        copiedTimer = setTimeout(() => (copied.value = false), 2000)
    } catch {
        // No clipboard (insecure origin, denied): the log stays readable here.
    }
}
</script>

<template>
    <div class="playback-log">
        <p class="log-intro">
            What the player and the page went through — track loads, errors, stalls, recovery
            attempts, the page being hidden or frozen. When playback stops on its own, copy this
            into a bug report.
        </p>
        <div class="log-actions">
            <Button
                class="playback-log-copy"
                size="small"
                severity="secondary"
                :label="copied ? 'Copied' : 'Copy'"
                :icon="copied ? 'ms-check' : 'ms-content-copy'"
                :disabled="playbackLog.length === 0"
                @click="copyLog"
            />
            <Button
                class="playback-log-clear"
                size="small"
                severity="secondary"
                text
                label="Clear"
                :disabled="playbackLog.length === 0"
                @click="clearPlaybackLog"
            />
        </div>
        <p v-if="playbackLog.length === 0" class="log-empty">Nothing recorded yet.</p>
        <ol v-else class="log-list">
            <li v-for="(entry, i) in newestFirst" :key="playbackLog.length - i" class="log-entry">
                <time class="log-time">{{ formatTime(entry.at) }}</time>
                <span class="log-event">{{ entry.event }}</span>
                <span v-if="entry.detail" class="log-detail">{{ entry.detail }}</span>
            </li>
        </ol>
    </div>
</template>

<style scoped>
.log-intro,
.log-empty {
    margin: 0;
    color: var(--app-text-secondary);
}

.log-actions {
    display: flex;
    gap: 0.5rem;
    margin: 0.75rem 0;
}

.log-empty {
    font-size: 0.9rem;
}

.log-list {
    list-style: none;
    margin: 0;
    padding: 0.5rem 0.75rem;
    max-height: 20rem;
    overflow-y: auto;
    border: 1px solid var(--app-border);
    border-radius: 6px;
    background: var(--app-surface);
    font-family: monospace;
    font-size: 0.8rem;
}

.log-entry {
    display: flex;
    flex-wrap: wrap;
    gap: 0 0.75rem;
    padding: 0.15rem 0;
}

.log-time {
    color: var(--app-text-secondary);
}

.log-event {
    color: var(--app-text-primary);
    font-weight: 600;
}

.log-detail {
    color: var(--app-text-secondary);
    min-width: 0;
    overflow-wrap: anywhere;
}
</style>
