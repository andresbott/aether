import { apiClient } from '@/lib/api/client'
import type {
    ListTasksResponse,
    ListExecutionsResponse,
    TriggerTaskResponse,
    TaskWithSchedule,
    ExecutionInfo,
    TaskSchedule,
    CreateScheduleBody,
    PatchScheduleBody
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

export async function listExecutions(signal?: AbortSignal): Promise<ExecutionInfo[]> {
    const { data } = await apiClient.get<ListExecutionsResponse>(`${TASKS_PATH}/executions`, { signal })
    return data.executions ?? []
}

export async function triggerTask(name: string): Promise<TriggerTaskResponse> {
    const { data } = await apiClient.post<TriggerTaskResponse>(
        `${TASKS_PATH}/${encodeURIComponent(name)}/trigger`
    )
    return data
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

export async function createSchedule(name: string, body: CreateScheduleBody): Promise<TaskSchedule> {
    const { data } = await apiClient.post<TaskSchedule>(
        `${TASKS_PATH}/${encodeURIComponent(name)}/schedules`,
        body
    )
    return data
}

export async function patchSchedule(name: string, id: string, body: PatchScheduleBody): Promise<TaskSchedule> {
    const { data } = await apiClient.patch<TaskSchedule>(
        `${TASKS_PATH}/${encodeURIComponent(name)}/schedules/${encodeURIComponent(id)}`,
        body
    )
    return data
}

export async function deleteSchedule(name: string, id: string): Promise<void> {
    await apiClient.delete(`${TASKS_PATH}/${encodeURIComponent(name)}/schedules/${encodeURIComponent(id)}`)
}
