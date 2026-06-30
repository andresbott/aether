export interface TaskDef {
    id: string
    name: string
    description: string
}

export interface TaskSchedule {
    id: number
    task_name: string
    cron_expression: string
    enabled: boolean
    created_at: string
    updated_at: string
}

export interface TaskWithSchedule extends TaskDef {
    schedule?: TaskSchedule | null
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
}

export interface UpsertTaskBody {
    cron_expression: string
    enabled?: boolean
}

export interface PatchTaskBody {
    cron_expression?: string
    enabled?: boolean
}
