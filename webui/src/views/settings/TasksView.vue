<script setup lang="ts">
import { ref, computed } from 'vue'
import Tabs from 'primevue/tabs'
import TabList from 'primevue/tablist'
import Tab from 'primevue/tab'
import TabPanels from 'primevue/tabpanels'
import TabPanel from 'primevue/tabpanel'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Message from 'primevue/message'
import { useToast } from 'primevue/usetoast'
import ExecutionHistory from '@/components/admin/ExecutionHistory.vue'
import LogViewer from '@/components/admin/LogViewer.vue'
import ScheduleDialog from '@/components/admin/ScheduleDialog.vue'
import { useTasks, EXECUTION_STATUS, SCHEDULE_PRESETS, progressLabel } from '@/composables/useTasks'
import type { Task } from '@/composables/useTasks'
import type { ExecutionInfo, CreateScheduleBody, PatchScheduleBody } from '@/types/tasks'
import { useViewport } from '@/composables/useViewport'

const toast = useToast()
const activeTab = ref('tasks')

const {
    tasks,
    executions,
    triggeringTaskId,
    tasksQuery,
    executionsQuery,
    triggerTask,
    cancelTaskExecution,
    cancelMutation,
    createSchedule,
    patchSchedule,
    deleteSchedule
} = useTasks()

const isTaskRunning = (task: Task): boolean =>
    task.lastExecutionStatus === EXECUTION_STATUS.waiting ||
    task.lastExecutionStatus === EXECUTION_STATUS.running

const scheduleSummary = (task: Task): string => {
    const list = task.schedules ?? []
    if (list.length === 0) return 'Not scheduled'
    if (list.length === 1) {
        const s = list[0]
        const label = SCHEDULE_PRESETS.find((p) => p.cron === s.cron_expression)?.label ?? s.cron_expression
        return s.enabled ? label : `${label} (paused)`
    }
    return `${list.length} schedules`
}

// Schedule dialog
// `tasks` is rebuilt into brand-new objects on every refetch
// (deriveTasksWithLastExecution maps to fresh Task instances), so holding a
// snapshot Task ref here would freeze the dialog on stale `schedules` after a
// create/patch/remove invalidates the query. Track only the id and look the
// current task back up reactively, so the open dialog re-renders with the
// live schedules list.
const scheduleDialogVisible = ref(false)
const scheduleDialogTaskId = ref<string | null>(null)
const scheduleDialogTask = computed<Task | null>(
    () => tasks.value.find((t) => t.id === scheduleDialogTaskId.value) ?? null
)
const scheduleSaving = ref(false)
const scheduleRemovingId = ref<string | null>(null)
const scheduleDialogRef = ref<InstanceType<typeof ScheduleDialog> | null>(null)

const openSchedule = (task: Task) => {
    scheduleDialogTaskId.value = task.id
    scheduleDialogVisible.value = true
}

const onScheduleCreate = async (body: CreateScheduleBody) => {
    const taskId = scheduleDialogTaskId.value
    if (!taskId) return
    scheduleSaving.value = true
    try {
        await createSchedule(taskId, body)
        // Reset the add form on confirmed success, not by inferring it from the
        // schedules list growing after a refetch (which may be slow or fail).
        scheduleDialogRef.value?.resetAddForm()
    } catch (e) {
        toast.add({ severity: 'error', summary: 'Failed to save schedule', detail: (e as Error).message, life: 5000 })
    } finally {
        scheduleSaving.value = false
    }
}

const onSchedulePatch = async (payload: { id: string; body: PatchScheduleBody }) => {
    const taskId = scheduleDialogTaskId.value
    if (!taskId) return
    scheduleSaving.value = true
    try {
        await patchSchedule(taskId, payload.id, payload.body)
    } catch (e) {
        toast.add({ severity: 'error', summary: 'Failed to save schedule', detail: (e as Error).message, life: 5000 })
    } finally {
        scheduleSaving.value = false
    }
}

const onScheduleRemove = async (id: string) => {
    const taskId = scheduleDialogTaskId.value
    if (!taskId) return
    scheduleRemovingId.value = id
    try {
        await deleteSchedule(taskId, id)
    } catch (e) {
        toast.add({ severity: 'error', summary: 'Failed to remove schedule', detail: (e as Error).message, life: 5000 })
    } finally {
        scheduleRemovingId.value = null
    }
}

// Log dialog
const logDialogVisible = ref(false)
const logExecutionId = ref('')
const openLog = (execution: ExecutionInfo) => {
    logExecutionId.value = execution.id
    logDialogVisible.value = true
}

const { tier } = useViewport()
// Spec §5: settings tables must not overflow a phone; these columns are the
// ones a phone admin can live without (the row's dialog still shows all).
const phoneCols = computed(() => tier.value === 'phone')
</script>

<template>
    <div class="tasks-view">
        <div class="header">
            <h1>Tasks</h1>
            <p>Manage and monitor background tasks and their execution queue</p>
        </div>

        <Message
            v-if="tasksQuery.isError.value || executionsQuery.isError.value"
            severity="error"
            :closable="false"
            class="mb-3"
        >
            {{
                tasksQuery.error.value?.message ||
                executionsQuery.error.value?.message ||
                'Failed to load tasks'
            }}
        </Message>

        <Tabs v-model:value="activeTab">
            <TabList>
                <Tab value="tasks">Tasks</Tab>
                <Tab value="queue">Queue</Tab>
            </TabList>
            <TabPanels>
                <TabPanel value="tasks">
                    <div class="table-fit">
                        <DataTable
                            :value="tasks"
                            :loading="tasksQuery.isLoading.value"
                            dataKey="id"
                            stripedRows
                        >
                            <template #empty><div class="empty-table">No tasks registered</div></template>
                            <Column header="Name">
                                <template #body="{ data }">
                                    <span class="task-name">{{ data.name }}</span>
                                    <p v-if="data.description" class="task-desc">{{ data.description }}</p>
                                    <p
                                        v-if="isTaskRunning(data) && data.lastExecutionProgress?.stage"
                                        class="task-stage"
                                    >{{ data.lastExecutionProgress.stage }}</p>
                                </template>
                            </Column>
                            <Column header="Schedule" :hidden="phoneCols" style="width: 10rem">
                                <template #body="{ data }">
                                    <span class="schedule-summary">{{ scheduleSummary(data) }}</span>
                                </template>
                            </Column>
                            <Column style="width: 4rem">
                                <template #body="{ data }">
                                    <Button
                                        icon="ms-calendar-today"
                                        text
                                        rounded
                                        size="small"
                                        :aria-label="data.schedules?.length ? 'Edit schedules' : 'Schedule'"
                                        @click.stop="openSchedule(data)"
                                    />
                                </template>
                            </Column>
                            <Column header="Actions" style="width: 11rem">
                                <template #body="{ data }">
                                    <Button
                                        :label="isTaskRunning(data) ? progressLabel(data.lastExecutionStatus, data.lastExecutionProgress) : 'Run'"
                                        :icon="isTaskRunning(data) ? undefined : 'ms-play-arrow'"
                                        :class="{ 'task-progress': isTaskRunning(data) }"
                                        size="small"
                                        :loading="triggeringTaskId === data.id || isTaskRunning(data)"
                                        :disabled="triggeringTaskId !== null || isTaskRunning(data)"
                                        @click.stop="triggerTask(data)"
                                    />
                                </template>
                            </Column>
                        </DataTable>
                    </div>
                </TabPanel>

                <TabPanel value="queue">
                    <ExecutionHistory
                        :executions="executions"
                        :isLoading="executionsQuery.isLoading.value"
                        :canceling="cancelMutation.isPending.value"
                        @cancel="cancelTaskExecution"
                        @row-click="openLog"
                    />
                </TabPanel>
            </TabPanels>
        </Tabs>

        <ScheduleDialog
            ref="scheduleDialogRef"
            v-model:visible="scheduleDialogVisible"
            :task="scheduleDialogTask"
            :saving="scheduleSaving"
            :removingId="scheduleRemovingId"
            @create="onScheduleCreate"
            @patch="onSchedulePatch"
            @remove="onScheduleRemove"
        />

        <LogViewer v-model:visible="logDialogVisible" :executionId="logExecutionId" />
    </div>
</template>

<style scoped>
.tasks-view {
    display: flex;
    flex-direction: column;
    padding: 2rem;
    overflow-y: auto;
}
.header h1 {
    font-size: 1.5rem;
    font-weight: 700;
    margin: 0 0 0.25rem;
}
.header p {
    color: var(--app-text-secondary);
    margin: 0 0 1.5rem;
}
.task-name {
    font-weight: 600;
}
.task-desc {
    margin: 0.25rem 0 0;
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}
.task-stage {
    margin: 0.25rem 0 0;
    font-size: 0.8rem;
    font-style: italic;
    color: var(--app-text-secondary);
}
.schedule-summary {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}
.empty-table {
    text-align: center;
    padding: 2rem;
    color: var(--app-text-secondary);
}
.mb-3 {
    margin-bottom: 1rem;
}
.table-fit {
    overflow-x: auto;
}
</style>
