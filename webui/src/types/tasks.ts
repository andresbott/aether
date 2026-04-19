export interface TaskDef {
    id: string
    name: string
    description: string
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
    tasks: TaskDef[]
}

export interface ListExecutionsResponse {
    executions: ExecutionInfo[]
}

export interface TriggerTaskResponse {
    execution_id: string
}
