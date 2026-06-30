import { ref, computed, unref } from 'vue'
import type { Ref } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as TasksApi from '@/lib/api/Tasks'
import type { TaskWithSchedule, ExecutionInfo, UpsertTaskBody, PatchTaskBody } from '@/types/tasks'

export const TASKS_QUERY_KEY = ['tasks'] as const
export const EXECUTIONS_QUERY_KEY = ['tasks', 'executions'] as const

const EXECUTIONS_POLL_INTERVAL_MS = 500

export const EXECUTION_STATUS = {
    waiting: 'waiting',
    running: 'running',
    complete: 'complete',
    failed: 'failed',
    panicked: 'panicked',
    canceled: 'canceled',
    cancel_error: 'cancel_error'
} as const

// Quartz cron: second minute hour day-of-month month day-of-week (6 fields)
export const SCHEDULE_PRESETS = [
    { label: 'Daily', cron: '0 0 0 * * *' },
    { label: 'Weekly', cron: '0 0 0 * * 1' },
    { label: 'Monthly', cron: '0 0 0 1 * *' }
] as const

export type ExecutionSeverity = 'info' | 'warn' | 'success' | 'danger' | 'secondary' | 'contrast'

export function getStatusSeverity(status: string): ExecutionSeverity {
    switch (status) {
        case EXECUTION_STATUS.complete:
            return 'success'
        case EXECUTION_STATUS.failed:
        case EXECUTION_STATUS.panicked:
        case EXECUTION_STATUS.cancel_error:
            return 'danger'
        case EXECUTION_STATUS.waiting:
            return 'warn'
        case EXECUTION_STATUS.running:
            return 'info'
        case EXECUTION_STATUS.canceled:
            return 'contrast'
        default:
            return 'secondary'
    }
}

export function getStatusLabel(status: string): string {
    return status === EXECUTION_STATUS.waiting ? 'queued' : status
}

export function isActiveStatus(status: string): boolean {
    return status === EXECUTION_STATUS.waiting || status === EXECUTION_STATUS.running
}

export function hasActiveExecutions(executions: ExecutionInfo[] | undefined): boolean {
    if (!executions?.length) return false
    return executions.some((e) => isActiveStatus(e.status))
}

export interface Task extends TaskWithSchedule {
    lastExecution: string | null
    lastExecutionStatus: string | null
}

export function deriveTasksWithLastExecution(
    tasks: TaskWithSchedule[],
    executions: ExecutionInfo[]
): Task[] {
    const byTask = new Map<string, ExecutionInfo[]>()
    for (const e of executions) {
        const list = byTask.get(e.task_name) ?? []
        list.push(e)
        byTask.set(e.task_name, list)
    }
    return tasks.map((t) => {
        const list = (byTask.get(t.id) ?? []).sort(
            (a, b) => new Date(b.queued_at).getTime() - new Date(a.queued_at).getTime()
        )
        const last = list[0]
        return {
            ...t,
            lastExecution: last ? last.started_at || last.queued_at : null,
            lastExecutionStatus: last?.status ?? null
        }
    })
}

export function useTasks() {
    const queryClient = useQueryClient()
    const toast = useToast()
    const triggeringTaskId = ref<string | null>(null)

    const tasksQuery = useQuery({
        queryKey: TASKS_QUERY_KEY,
        queryFn: TasksApi.listTasks,
        staleTime: 60 * 1000
    })

    const executionsQuery = useQuery({
        queryKey: EXECUTIONS_QUERY_KEY,
        queryFn: TasksApi.listExecutions,
        refetchInterval: (query) =>
            hasActiveExecutions(query.state.data) ? EXECUTIONS_POLL_INTERVAL_MS : false,
        refetchIntervalInBackground: false
    })

    const tasks = computed<Task[]>(() =>
        deriveTasksWithLastExecution(tasksQuery.data.value ?? [], executionsQuery.data.value ?? [])
    )
    const executions = computed<ExecutionInfo[]>(() => executionsQuery.data.value ?? [])

    const triggerMutation = useMutation({
        mutationFn: (name: string) => TasksApi.triggerTask(name),
        onMutate: (name: string) => {
            triggeringTaskId.value = name
        },
        onError: (error: Error) => {
            toast.add({ severity: 'error', summary: 'Task trigger failed', detail: error.message, life: 5000 })
        },
        onSettled: () => {
            triggeringTaskId.value = null
            queryClient.invalidateQueries({ queryKey: EXECUTIONS_QUERY_KEY })
        }
    })

    const cancelMutation = useMutation({
        mutationFn: (executionId: string) => TasksApi.cancelExecution(executionId),
        onSettled: () => {
            queryClient.invalidateQueries({ queryKey: EXECUTIONS_QUERY_KEY })
        }
    })

    const upsertMutation = useMutation({
        mutationFn: ({ name, body }: { name: string; body: UpsertTaskBody }) => TasksApi.upsertTask(name, body),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: TASKS_QUERY_KEY })
        }
    })
    const patchMutation = useMutation({
        mutationFn: ({ name, body }: { name: string; body: PatchTaskBody }) => TasksApi.patchTask(name, body),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: TASKS_QUERY_KEY })
        }
    })
    const deleteMutation = useMutation({
        mutationFn: (name: string) => TasksApi.deleteTaskSchedule(name),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: TASKS_QUERY_KEY })
        }
    })

    return {
        tasks,
        executions,
        triggeringTaskId,
        tasksQuery,
        executionsQuery,
        triggerTask: (task: Task) => triggerMutation.mutate(task.id),
        cancelTaskExecution: (executionId: string) => cancelMutation.mutate(executionId),
        cancelMutation,
        upsertTask: (name: string, body: UpsertTaskBody) => upsertMutation.mutateAsync({ name, body }),
        patchTask: (name: string, body: PatchTaskBody) => patchMutation.mutateAsync({ name, body }),
        deleteTaskSchedule: (name: string) => deleteMutation.mutateAsync(name),
        getStatusSeverity,
        getStatusLabel,
        getExecutionLog: (executionId: string) => TasksApi.getExecutionLog(executionId)
    }
}

export function useExecutionLog(executionId: string | Ref<string>, enabled: Ref<boolean>) {
    return useQuery({
        queryKey: computed(() => ['tasks', 'executions', unref(executionId), 'log']),
        queryFn: () => TasksApi.getExecutionLog(unref(executionId)),
        enabled,
        staleTime: 2 * 1000,
        refetchInterval: 5000
    })
}
