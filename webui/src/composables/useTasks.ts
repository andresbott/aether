import { ref, computed, unref } from 'vue'
import type { Ref } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as TasksApi from '@/lib/api/Tasks'
import type {
    TaskWithSchedule,
    ExecutionInfo,
    ExecutionProgress,
    CreateScheduleBody,
    PatchScheduleBody
} from '@/types/tasks'

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

// progressLabel is the running task's action-cell text: a percentage when the
// run reports a measurable total, else a plain state word. done/total are raw
// work units; the percentage is derived here so the API carries no float.
export function progressLabel(status: string | null, progress: ExecutionProgress | null): string {
    if (progress && progress.total > 0) {
        return `${Math.round((progress.done / progress.total) * 100)}% complete`
    }
    if (status === EXECUTION_STATUS.waiting) return 'Queued'
    return 'Running'
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
    lastExecutionProgress: ExecutionProgress | null
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
            lastExecutionStatus: last?.status ?? null,
            lastExecutionProgress: last?.progress ?? null
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
        queryFn: ({ signal }) => TasksApi.listExecutions(signal),
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
        onSuccess: (result) => {
            // A singleton task (scan, scan-full) coalesces: a trigger while one
            // is already waiting/running enqueues nothing and returns the
            // in-flight run. Tell the user rather than silently doing nothing.
            if (result.reused) {
                toast.add({
                    severity: 'info',
                    summary: 'Already in progress',
                    detail: 'This task is already queued or running — showing the current run.',
                    life: 4000
                })
            }
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

    const createScheduleMutation = useMutation({
        mutationFn: ({ name, body }: { name: string; body: CreateScheduleBody }) =>
            TasksApi.createSchedule(name, body),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: TASKS_QUERY_KEY })
    })
    const patchScheduleMutation = useMutation({
        mutationFn: ({ name, id, body }: { name: string; id: string; body: PatchScheduleBody }) =>
            TasksApi.patchSchedule(name, id, body),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: TASKS_QUERY_KEY })
    })
    const deleteScheduleMutation = useMutation({
        mutationFn: ({ name, id }: { name: string; id: string }) => TasksApi.deleteSchedule(name, id),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: TASKS_QUERY_KEY })
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
        createSchedule: (name: string, body: CreateScheduleBody) =>
            createScheduleMutation.mutateAsync({ name, body }),
        patchSchedule: (name: string, id: string, body: PatchScheduleBody) =>
            patchScheduleMutation.mutateAsync({ name, id, body }),
        deleteSchedule: (name: string, id: string) => deleteScheduleMutation.mutateAsync({ name, id }),
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
