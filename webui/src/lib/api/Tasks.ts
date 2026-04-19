import { apiClient } from '@/lib/api/client'
import type {
    ListTasksResponse,
    ListExecutionsResponse,
    TriggerTaskResponse,
    TaskDef,
    ExecutionInfo
} from '@/types/tasks'

export async function listTasks(): Promise<TaskDef[]> {
    const { data } = await apiClient.get<ListTasksResponse>('/tasks')
    return data.tasks ?? []
}

export async function triggerTask(name: string): Promise<string> {
    const { data } = await apiClient.post<TriggerTaskResponse>(
        `/tasks/${encodeURIComponent(name)}`
    )
    return data.execution_id
}

export async function listExecutions(taskName: string): Promise<ExecutionInfo[]> {
    const { data } = await apiClient.get<ListExecutionsResponse>(
        `/tasks/${encodeURIComponent(taskName)}/executions`
    )
    return data.executions ?? []
}

export async function cancelExecution(
    taskName: string,
    executionId: string
): Promise<void> {
    await apiClient.delete(
        `/tasks/${encodeURIComponent(taskName)}/executions/${encodeURIComponent(executionId)}`
    )
}

export async function getExecutionLog(
    taskName: string,
    executionId: string
): Promise<string> {
    const { data } = await apiClient.get<string>(
        `/tasks/${encodeURIComponent(taskName)}/executions/${encodeURIComponent(executionId)}/log`,
        { responseType: 'text' }
    )
    return data ?? ''
}
