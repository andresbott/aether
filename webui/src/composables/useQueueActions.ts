import { ref, computed } from 'vue'
import { useToast } from 'primevue/usetoast'
import { usePlayer } from '@/composables/usePlayer'
import { useCreatePlaylist } from '@/composables/useSubsonicQueries'

export function useQueueActions() {
    const player = usePlayer()
    const toast = useToast()
    const createPlaylist = useCreatePlaylist()

    const showSaveDialog = ref(false)
    const playlistName = ref('')

    const openSaveDialog = (): void => {
        playlistName.value = ''
        showSaveDialog.value = true
    }

    const handleSave = (): void => {
        const name = playlistName.value.trim()
        if (!name) return
        const songIds = player.queue.value.map(s => s.id)
        createPlaylist.mutate(
            { name, songIds },
            {
                onSuccess: () => {
                    showSaveDialog.value = false
                    playlistName.value = ''
                    toast.add({
                        severity: 'success',
                        summary: 'Playlist created',
                        detail: name,
                        life: 3000
                    })
                },
                onError: (err: Error) => {
                    toast.add({
                        severity: 'error',
                        summary: 'Failed to save playlist',
                        detail: err.message,
                        life: 5000
                    })
                }
            }
        )
    }

    const isSaving = computed(() => createPlaylist.isPending.value)

    const clearQueue = (): void => {
        player.clearQueue()
    }

    return { showSaveDialog, playlistName, openSaveDialog, handleSave, isSaving, clearQueue }
}
