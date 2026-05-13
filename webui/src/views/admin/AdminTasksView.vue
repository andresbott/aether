<script setup lang="ts">
import { ref, computed } from 'vue'
import Button from 'primevue/button'
import SelectButton from 'primevue/selectbutton'
import ExecutionHistory from '@/components/admin/ExecutionHistory.vue'
import { useTasks, useExecutions, useTriggerTask } from '@/composables/useTasks'
import type { ExecutionInfo } from '@/types/tasks'

const { data: tasks, isLoading: tasksLoading } = useTasks()
const triggerMutation = useTriggerTask()

const selectedTaskFilter = ref<string>('all')

const taskFilterOptions = computed(() => {
    const options = [{ label: 'All', value: 'all' }]
    if (tasks.value) {
        tasks.value.forEach((t) => {
            options.push({ label: t.name, value: t.id })
        })
    }
    return options
})

const firstTaskId = computed(() => tasks.value?.[0]?.id ?? '')

const { data: executions, isLoading: executionsLoading } = useExecutions(
    computed(() => (selectedTaskFilter.value === 'all' ? firstTaskId.value : selectedTaskFilter.value))
)

const filteredExecutions = computed<ExecutionInfo[]>(() => {
    if (!executions.value) return []
    if (selectedTaskFilter.value === 'all') return executions.value
    return executions.value.filter((e) => e.task_name === selectedTaskFilter.value)
})

const isTaskActive = (taskId: string): boolean => {
    if (!executions.value) return false
    return executions.value.some(
        (e) =>
            e.task_name === taskId &&
            (e.status === 'Queued' || e.status === 'Running')
    )
}

const triggerTask = (taskId: string) => {
    triggerMutation.mutate(taskId)
}
</script>

<template>
    <div class="tasks-view">
        <section class="section">
            <h2>Tasks</h2>
            <div v-if="tasksLoading" class="loading">
                <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
            </div>
            <div v-else-if="tasks && tasks.length > 0" class="task-cards">
                <div v-for="task in tasks" :key="task.id" class="task-card">
                    <div class="task-info">
                        <h3>{{ task.name }}</h3>
                        <p v-if="task.description">{{ task.description }}</p>
                    </div>
                    <Button
                        label="Run"
                        icon="pi pi-play"
                        :disabled="isTaskActive(task.id) || triggerMutation.isPending.value"
                        :loading="triggerMutation.isPending.value"
                        @click="triggerTask(task.id)"
                    />
                </div>
            </div>
            <div v-else class="empty-state">
                <p>No tasks registered</p>
            </div>
        </section>

        <section class="section">
            <div class="section-header">
                <h2>Execution History</h2>
                <SelectButton
                    v-if="taskFilterOptions.length > 2"
                    v-model="selectedTaskFilter"
                    :options="taskFilterOptions"
                    optionLabel="label"
                    optionValue="value"
                    :allowEmpty="false"
                />
            </div>
            <ExecutionHistory
                :executions="filteredExecutions"
                :isLoading="executionsLoading"
            />
        </section>
    </div>
</template>

<style scoped>
.tasks-view {
    display: flex;
    flex-direction: column;
    padding: 2rem;
    overflow-y: auto;
}

.section {
    margin-bottom: 2.5rem;
}

.section h2 {
    font-size: 1.25rem;
    font-weight: 600;
    margin-bottom: 1rem;
}

.section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1rem;
}

.loading {
    display: flex;
    justify-content: center;
    padding: 2rem;
    color: var(--app-text-secondary);
}

.task-cards {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
}

.task-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1rem 1.25rem;
    background-color: var(--app-surface);
    border: 1px solid var(--app-border);
    border-radius: 8px;
}

.task-info h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
}

.task-info p {
    margin: 0.25rem 0 0;
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}

.empty-state {
    text-align: center;
    padding: 2rem;
    color: var(--app-text-secondary);
}
</style>
