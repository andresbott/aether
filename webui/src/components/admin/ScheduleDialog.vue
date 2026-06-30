<script setup lang="ts">
import { ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Checkbox from 'primevue/checkbox'
import { SCHEDULE_PRESETS, type Task } from '@/composables/useTasks'

const props = defineProps<{
    visible: boolean
    task: Task | null
    saving?: boolean
}>()

const emit = defineEmits<{
    'update:visible': [value: boolean]
    save: [payload: { cron_expression: string; enabled: boolean }]
    remove: []
}>()

const cronExpression = ref('')
const enabled = ref(true)
const error = ref('')

watch(
    () => props.visible,
    (val) => {
        if (val) {
            cronExpression.value = props.task?.schedule?.cron_expression ?? ''
            enabled.value = props.task?.schedule?.enabled ?? true
            error.value = ''
        }
    },
    { immediate: true }
)

const setPreset = (cron: string) => {
    cronExpression.value = cron
    error.value = ''
}

const onSave = () => {
    const cron = cronExpression.value.trim()
    if (!cron) {
        error.value = 'Cron expression is required'
        return
    }
    emit('save', { cron_expression: cron, enabled: enabled.value })
}
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="emit('update:visible', $event)"
        :header="task ? `Schedule: ${task.name}` : 'Schedule'"
        modal
        :style="{ width: '480px' }"
    >
        <div class="schedule-form">
            <div class="presets">
                <span class="preset-label">Preset:</span>
                <Button
                    v-for="p in SCHEDULE_PRESETS"
                    :key="p.cron"
                    :label="p.label"
                    size="small"
                    :severity="cronExpression === p.cron ? 'primary' : 'secondary'"
                    @click="setPreset(p.cron)"
                />
            </div>
            <div class="field">
                <label for="schedule-cron">Cron expression</label>
                <InputText
                    id="schedule-cron"
                    v-model="cronExpression"
                    placeholder="e.g. 0 0 0 * * *"
                    class="w-full"
                    @input="error = ''"
                />
            </div>
            <div class="field-inline">
                <Checkbox v-model="enabled" :binary="true" inputId="schedule-enabled" />
                <label for="schedule-enabled">Enabled</label>
            </div>
            <p v-if="error" class="error">{{ error }}</p>
        </div>
        <template #footer>
            <Button
                v-if="task?.schedule"
                label="Remove schedule"
                severity="danger"
                text
                :disabled="saving"
                @click="emit('remove')"
            />
            <span class="spacer" />
            <Button label="Cancel" text severity="secondary" @click="emit('update:visible', false)" />
            <Button label="Save" icon="pi pi-check" :loading="saving" @click="onSave" />
        </template>
    </Dialog>
</template>

<style scoped>
.schedule-form {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    padding: 0.5rem 0;
}
.presets {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: center;
}
.preset-label {
    font-weight: 600;
}
.field {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
}
.field :deep(input) {
    width: 100%;
}
.field-inline {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}
.error {
    color: var(--p-red-500, #ef4444);
    margin: 0;
    font-size: 0.85rem;
}
.spacer {
    flex: 1 1 auto;
}
</style>
