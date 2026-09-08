<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Checkbox from 'primevue/checkbox'
import Tag from 'primevue/tag'
import { SCHEDULE_PRESETS, type Task } from '@/composables/useTasks'
import type { CreateScheduleBody, PatchScheduleBody, TaskSchedule } from '@/types/tasks'

const props = defineProps<{
    visible: boolean
    task: Task | null
    saving?: boolean
}>()

const emit = defineEmits<{
    'update:visible': [value: boolean]
    create: [body: CreateScheduleBody]
    patch: [payload: { id: string; body: PatchScheduleBody }]
    remove: [id: string]
}>()

const isScan = computed(() => props.task?.id === 'scan')

const schedules = computed<TaskSchedule[]>(() => props.task?.schedules ?? [])

// Edit buffer: null = the "add new" form; otherwise the id of the schedule
// being edited.
const editingId = ref<string | null>(null)
const cronExpression = ref('')
const enabled = ref(true)
const full = ref(false)
const error = ref('')

const resetForm = () => {
    editingId.value = null
    cronExpression.value = ''
    enabled.value = true
    full.value = false
    error.value = ''
}

watch(
    () => props.visible,
    (v) => {
        if (v) resetForm()
    },
    { immediate: true }
)

const scheduleLabel = (s: TaskSchedule): string => {
    const preset = SCHEDULE_PRESETS.find((p) => p.cron === s.cron_expression)
    return preset ? preset.label : s.cron_expression
}

const isFullSchedule = (s: TaskSchedule): boolean => Boolean((s.params as { full?: boolean } | undefined)?.full)

const setPreset = (cron: string) => {
    cronExpression.value = cron
    error.value = ''
}

const editRow = (s: TaskSchedule) => {
    editingId.value = s.id
    cronExpression.value = s.cron_expression
    enabled.value = s.enabled
    full.value = isFullSchedule(s)
    error.value = ''
}

const cancelEdit = () => {
    resetForm()
}

const onSave = () => {
    const cron = cronExpression.value.trim()
    if (!cron) {
        error.value = 'Cron expression is required'
        return
    }
    const params = isScan.value ? { full: full.value } : undefined
    if (editingId.value) {
        emit('patch', { id: editingId.value, body: { cron_expression: cron, enabled: enabled.value, params } })
    } else {
        emit('create', { cron_expression: cron, enabled: enabled.value, params })
    }
}
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="emit('update:visible', $event)"
        :header="task ? `Schedules: ${task.name}` : 'Schedules'"
        modal
        :style="{ width: '560px' }"
    >
        <div class="schedule-manager">
            <div v-if="schedules.length" class="schedule-list">
                <div v-for="s in schedules" :key="s.id" class="schedule-row">
                    <div class="schedule-row-info">
                        <span class="schedule-row-label">{{ scheduleLabel(s) }}</span>
                        <Tag v-if="isScan" :severity="isFullSchedule(s) ? 'warn' : 'secondary'" :value="isFullSchedule(s) ? 'Full' : 'Incremental'" />
                        <span v-if="!s.enabled" class="schedule-row-paused">(paused)</span>
                    </div>
                    <div class="schedule-row-actions">
                        <Button icon="pi pi-pencil" text rounded size="small" aria-label="Edit schedule" @click="editRow(s)" />
                        <Button
                            icon="pi pi-trash"
                            text
                            rounded
                            size="small"
                            severity="danger"
                            aria-label="Remove schedule"
                            :disabled="saving"
                            @click="emit('remove', s.id)"
                        />
                    </div>
                </div>
            </div>
            <p v-else class="no-schedules">No schedules yet.</p>

            <div class="schedule-form">
                <h3 class="form-title">{{ editingId ? 'Edit schedule' : 'Add schedule' }}</h3>
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
                <div v-if="isScan" class="field-inline">
                    <Checkbox v-model="full" :binary="true" inputId="schedule-full" />
                    <label for="schedule-full">Full scan</label>
                </div>
                <p v-if="error" class="error">{{ error }}</p>
                <div class="form-actions">
                    <Button v-if="editingId" label="Cancel edit" text severity="secondary" :disabled="saving" @click="cancelEdit" />
                    <Button :label="editingId ? 'Save changes' : 'Add schedule'" icon="pi pi-check" :loading="saving" @click="onSave" />
                </div>
            </div>
        </div>
        <template #footer>
            <Button label="Close" text severity="secondary" @click="emit('update:visible', false)" />
        </template>
    </Dialog>
</template>

<style scoped>
.schedule-manager {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
    padding: 0.5rem 0;
}
.schedule-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}
.schedule-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--p-content-border-color, #e5e7eb);
    border-radius: 6px;
}
.schedule-row-info {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
}
.schedule-row-label {
    font-weight: 600;
}
.schedule-row-paused {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
}
.schedule-row-actions {
    display: flex;
    align-items: center;
    gap: 0.25rem;
}
.no-schedules {
    color: var(--app-text-secondary);
    margin: 0;
}
.schedule-form {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    padding-top: 0.5rem;
    border-top: 1px solid var(--p-content-border-color, #e5e7eb);
}
.form-title {
    font-size: 1rem;
    font-weight: 600;
    margin: 0;
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
.form-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
}
</style>
