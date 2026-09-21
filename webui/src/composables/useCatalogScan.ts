import { computed, watch } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as TasksApi from '@/lib/api/Tasks'
import { apiErrorMessage } from '@/lib/apiError'
import {
    useExecutions,
    isActiveStatus,
    progressLabel,
    EXECUTION_STATUS,
    EXECUTIONS_QUERY_KEY
} from '@/composables/useTasks'
import { scanFolderQueryKeys } from '@/composables/useScanFolders'
import { libraryQueryKeys } from '@/composables/useLibraries'
import type { ExecutionInfo } from '@/types/tasks'

// The incremental "Catalog Scan" task (ScanTaskName in app/tasks/scan.go).
export const CATALOG_SCAN_TASK = 'scan'

// useCatalogScan starts the incremental catalog scan and follows it: whether a
// run is queued or running — whoever started it: this button, the Tasks view or
// a schedule — how far it has got, and what to refresh once it settles.
export function useCatalogScan() {
    const queryClient = useQueryClient()
    const toast = useToast()
    const executionsQuery = useExecutions()

    // The scan task is a coalescing singleton, so at most one run is active.
    const activeRun = computed(() =>
        executionsQuery.data.value?.find(
            (e) => e.task_name === CATALOG_SCAN_TASK && isActiveStatus(e.status)
        )
    )
    const running = computed(() => activeRun.value !== undefined)
    const progressText = computed(() =>
        activeRun.value ? progressLabel(activeRun.value.status, activeRun.value.progress ?? null) : ''
    )

    // Runs seen queued or running, plus the one a trigger hands back: a small
    // scan can finish before any poll sees it running. A run that had already
    // settled when the page loaded is never in here, so it refreshes nothing.
    const inFlight = new Set<string>()

    function track(executions: ExecutionInfo[] | undefined) {
        for (const e of executions ?? []) {
            if (e.task_name !== CATALOG_SCAN_TASK) continue
            if (isActiveStatus(e.status)) inFlight.add(e.id)
            else if (inFlight.delete(e.id)) settled(e.status)
        }
    }

    function settled(status: string) {
        // A scan changes the per-folder and per-library track counts, the
        // library filters' pick-lists and the catalog every music view shows.
        queryClient.invalidateQueries({ queryKey: scanFolderQueryKeys.all })
        queryClient.invalidateQueries({ queryKey: libraryQueryKeys.all })
        queryClient.invalidateQueries({ queryKey: ['subsonic'] })
        if (status !== EXECUTION_STATUS.complete && status !== EXECUTION_STATUS.canceled) {
            toast.add({
                severity: 'error',
                summary: 'Catalog scan failed',
                detail: 'Its log is in the Queue tab under Tasks.',
                life: 6000
            })
        }
    }

    watch(() => executionsQuery.data.value, track)

    const trigger = useMutation({
        mutationFn: () => TasksApi.triggerTask(CATALOG_SCAN_TASK),
        onSuccess: (result) => {
            inFlight.add(result.execution_id)
            // The run is a singleton: a trigger while one is queued or running
            // joins it rather than starting another.
            if (result.reused) {
                toast.add({
                    severity: 'info',
                    summary: 'Already in progress',
                    detail: 'A catalog scan is already queued or running.',
                    life: 4000
                })
            }
        },
        onError: (err) => {
            toast.add({
                severity: 'error',
                summary: 'Could not start the scan',
                detail: apiErrorMessage(err),
                life: 5000
            })
        },
        // Awaited, so `starting` lasts until the list shows the run. A poll
        // that raced the trigger may already hold the finished run, before its
        // id was tracked; the refetch then changes nothing for the watch to
        // see, so the cache is checked here as well.
        onSettled: async () => {
            await queryClient.invalidateQueries({ queryKey: EXECUTIONS_QUERY_KEY })
            track(queryClient.getQueryData<ExecutionInfo[]>(EXECUTIONS_QUERY_KEY))
        }
    })

    return {
        start: () => trigger.mutate(),
        starting: trigger.isPending,
        running,
        progressText
    }
}
