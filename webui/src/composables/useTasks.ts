import { computed, unref } from 'vue'
import type { Ref } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as TasksApi from '@/lib/api/Tasks'
import type { ExecutionInfo } from '@/types/tasks'

export const taskQueryKeys = {
    tasks: ['tasks'] as const,
    executions: (taskName: string) => ['tasks', taskName, 'executions'] as const,
    executionLog: (taskName: string, executionId: string) =>
        ['tasks', taskName, 'executions', executionId, 'log'] as const
}

export function useTasks() {
    return useQuery({
        queryKey: taskQueryKeys.tasks,
        queryFn: () => TasksApi.listTasks(),
        staleTime: 5 * 60 * 1000
    })
}

export function useExecutions(taskName: string | Ref<string>) {
    return useQuery<ExecutionInfo[]>({
        queryKey: computed(() => taskQueryKeys.executions(unref(taskName))),
        queryFn: () => TasksApi.listExecutions(unref(taskName)),
        staleTime: 5 * 1000,
        refetchInterval: (q): number | false => {
            const execs = q.state.data
            if (!execs) return false
            const hasActive = execs.some(
                (e: ExecutionInfo) => e.status === 'Queued' || e.status === 'Running'
            )
            return hasActive ? 5000 : false
        }
    })
}

export function useTriggerTask() {
    const queryClient = useQueryClient()
    const toast = useToast()

    return useMutation({
        mutationFn: (taskName: string) => TasksApi.triggerTask(taskName),
        onSuccess: (_id, taskName) => {
            queryClient.invalidateQueries({
                queryKey: taskQueryKeys.executions(taskName)
            })
            toast.add({
                severity: 'success',
                summary: 'Task triggered',
                detail: `"${taskName}" has been queued`,
                life: 3000
            })
        },
        onError: (error) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to trigger task',
                detail: error.message,
                life: 5000
            })
        }
    })
}

export function useCancelExecution() {
    const queryClient = useQueryClient()
    const toast = useToast()

    return useMutation({
        mutationFn: ({ taskName, executionId }: { taskName: string; executionId: string }) =>
            TasksApi.cancelExecution(taskName, executionId),
        onSuccess: (_data, { taskName }) => {
            queryClient.invalidateQueries({
                queryKey: taskQueryKeys.executions(taskName)
            })
            toast.add({
                severity: 'info',
                summary: 'Execution cancelled',
                life: 3000
            })
        }
    })
}

export function useExecutionLog(
    taskName: string | Ref<string>,
    executionId: string | Ref<string>,
    enabled: Ref<boolean>
) {
    return useQuery({
        queryKey: computed(() =>
            taskQueryKeys.executionLog(unref(taskName), unref(executionId))
        ),
        queryFn: () => TasksApi.getExecutionLog(unref(taskName), unref(executionId)),
        enabled,
        staleTime: 2 * 1000,
        refetchInterval: 5000
    })
}
