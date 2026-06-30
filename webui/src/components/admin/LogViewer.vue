<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import Dialog from 'primevue/dialog'
import { useExecutionLog } from '@/composables/useTasks'

const props = defineProps<{
    visible: boolean
    executionId: string
}>()

const emit = defineEmits<{
    'update:visible': [value: boolean]
}>()

const logContainer = ref<HTMLElement | null>(null)
const enabled = ref(false)

watch(
    () => props.visible,
    (val) => {
        enabled.value = val
    }
)

const executionIdRef = computed(() => props.executionId)
const { data: logText } = useExecutionLog(executionIdRef, enabled)

watch(logText, async () => {
    await nextTick()
    if (logContainer.value) {
        logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
})
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="emit('update:visible', $event)"
        header="Execution Log"
        modal
        :style="{ width: '700px', maxHeight: '80vh' }"
    >
        <div ref="logContainer" class="log-content">
            <pre v-if="logText">{{ logText }}</pre>
            <div v-else class="log-empty">
                <i class="pi pi-file" style="font-size: 1.5rem"></i>
                <p>No log output</p>
            </div>
        </div>
    </Dialog>
</template>

<style scoped>
.log-content {
    max-height: 60vh;
    overflow-y: auto;
    background-color: #1e1e2e;
    border-radius: 6px;
    padding: 1rem;
}
.log-content pre {
    margin: 0;
    font-family: 'Fira Code', 'Consolas', 'Courier New', monospace;
    font-size: 0.8rem;
    line-height: 1.6;
    color: #cdd6f4;
    white-space: pre-wrap;
    word-break: break-all;
}
.log-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
    padding: 2rem;
    color: #6c7086;
}
</style>
