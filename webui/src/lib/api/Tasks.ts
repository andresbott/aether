import { apiClient } from '@/lib/api/client'
import type {
    ListTasksResponse,
    ListExecutionsResponse,
    TriggerTaskResponse,
    TaskWithSchedule,
    ExecutionInfo,
    UpsertTaskBody,
    PatchTaskBody
} from '@/types/tasks'

const TASKS_PATH = '/tasks'

export async function listTasks(): Promise<TaskWithSchedule[]> {
    const { data } = await apiClient.get<ListTasksResponse>(TASKS_PATH)
    return data.tasks ?? []
}

export async function getTask(name: string): Promise<TaskWithSchedule> {
    const { data } = await apiClient.get<TaskWithSchedule>(`${TASKS_PATH}/${encodeURIComponent(name)}`)
    return data
}

export async function listExecutions(): Promise<ExecutionInfo[]> {
    const { data } = await apiClient.get<ListExecutionsResponse>(`${TASKS_PATH}/executions`)
    return data.executions ?? []
}

export async function triggerTask(name: string): Promise<string> {
    const { data } = await apiClient.post<TriggerTaskResponse>(
        `${TASKS_PATH}/${encodeURIComponent(name)}/trigger`
    )
    return data.execution_id
}

export async function cancelExecution(executionId: string): Promise<void> {
    await apiClient.post(`${TASKS_PATH}/executions/${encodeURIComponent(executionId)}/cancel`)
}

export async function getExecutionLog(executionId: string): Promise<string> {
    const { data } = await apiClient.get<string>(
        `${TASKS_PATH}/executions/${encodeURIComponent(executionId)}/logs`,
        { responseType: 'text' }
    )
    return data ?? ''
}

export async function upsertTask(name: string, body: UpsertTaskBody): Promise<TaskWithSchedule> {
    const { data } = await apiClient.put<TaskWithSchedule>(`${TASKS_PATH}/${encodeURIComponent(name)}`, body)
    return data
}

export async function patchTask(name: string, body: PatchTaskBody): Promise<TaskWithSchedule> {
    const { data } = await apiClient.patch<TaskWithSchedule>(`${TASKS_PATH}/${encodeURIComponent(name)}`, body)
    return data
}

export async function deleteTaskSchedule(name: string): Promise<void> {
    await apiClient.delete(`${TASKS_PATH}/${encodeURIComponent(name)}`)
}
