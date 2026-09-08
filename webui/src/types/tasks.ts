export interface TaskDef {
    id: string
    name: string
    description: string
}

export interface TaskSchedule {
    id: string
    task_name: string
    cron_expression: string
    params?: Record<string, unknown>
    enabled: boolean
    created_at: string
    updated_at: string
}

export interface TaskWithSchedule extends TaskDef {
    schedules: TaskSchedule[]
}

export interface ExecutionInfo {
    id: string
    task_name: string
    status: string
    queued_at: string
    started_at?: string
    ended_at: string
}

export interface ListTasksResponse {
    tasks: TaskWithSchedule[]
}

export interface ListExecutionsResponse {
    executions: ExecutionInfo[]
}

export interface TriggerTaskResponse {
    execution_id: string
    // True when the trigger coalesced onto a singleton task (scan) that was
    // already waiting or running: execution_id is that in-flight run's.
    reused: boolean
}

export interface CreateScheduleBody {
    cron_expression: string
    enabled?: boolean
    params?: Record<string, unknown>
}

export interface PatchScheduleBody {
    cron_expression?: string
    enabled?: boolean
    params?: Record<string, unknown>
}

export interface TriggerTaskBody {
    full?: boolean
}
