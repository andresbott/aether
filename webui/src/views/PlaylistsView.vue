<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import SelectButton from 'primevue/selectbutton'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import PlaylistCard from '@/components/library/PlaylistCard.vue'
import PlaylistListView from '@/components/library/PlaylistListView.vue'
import { usePlaylists, useCreatePlaylist } from '@/composables/useSubsonicQueries'

type Layout = 'grid' | 'list'

const route = useRoute()
const router = useRouter()
const { data: playlists, isLoading } = usePlaylists()
const createPlaylist = useCreatePlaylist()

const showCreateDialog = ref(false)
const newPlaylistName = ref('')

const layoutOptions = [
    { label: 'List', value: 'list', icon: 'pi pi-list' },
    { label: 'Grid', value: 'grid', icon: 'pi pi-th-large' }
]

const layout = computed<Layout>({
    get: () => (route.query.view === 'list' ? 'list' : 'grid'),
    set: (v) => {
        const query = { ...route.query }
        if (v === 'list') query.view = 'list'
        else delete query.view
        router.replace({ query })
    }
})

const summary = computed(() => {
    const count = playlists.value?.length ?? 0
    if (count === 0) return ''
    return `${count} ${count === 1 ? 'playlist' : 'playlists'}`
})

const handleCreate = () => {
    if (!newPlaylistName.value.trim()) return
    createPlaylist.mutate(
        { name: newPlaylistName.value.trim() },
        {
            onSuccess: () => {
                showCreateDialog.value = false
                newPlaylistName.value = ''
            }
        }
    )
}
</script>

<template>
    <ContentScaffold title="Playlists" :summary="summary">
        <template #actions>
            <SelectButton
                v-model="layout"
                :options="layoutOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
                dataKey="value"
                aria-label="Layout"
            >
                <template #option="slotProps">
                    <i :class="slotProps.option.icon"></i>
                </template>
            </SelectButton>
            <Button
                icon="pi pi-plus"
                text
                rounded
                v-tooltip.bottom="'Create playlist'"
                aria-label="Create playlist"
                @click="showCreateDialog = true"
            />
        </template>

        <div class="playlists-scroll">
            <div v-if="isLoading" class="loading">
                <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
            </div>

            <template v-else-if="playlists && playlists.length > 0">
                <PlaylistListView v-if="layout === 'list'" :playlists="playlists" />
                <div v-else class="playlist-grid">
                    <PlaylistCard v-for="pl in playlists" :key="pl.id" :playlist="pl" />
                </div>
            </template>

            <div v-else class="empty-state">
                <i class="pi pi-list" style="font-size: 3rem"></i>
                <p>No playlists</p>
            </div>
        </div>

        <Dialog
            v-model:visible="showCreateDialog"
            header="Create Playlist"
            :modal="true"
            :style="{ width: '400px' }"
        >
            <div class="create-form">
                <InputText
                    v-model="newPlaylistName"
                    placeholder="Playlist name"
                    class="w-full"
                    @keyup.enter="handleCreate"
                />
            </div>
            <template #footer>
                <Button label="Cancel" text @click="showCreateDialog = false" />
                <Button label="Create" :loading="createPlaylist.isPending.value" @click="handleCreate" />
            </template>
        </Dialog>
    </ContentScaffold>
</template>

<style scoped>
.playlists-scroll { height: 100%; overflow-y: auto; scrollbar-gutter: stable; }
.loading { display: flex; justify-content: center; padding: 3rem; color: var(--app-text-secondary); }
.playlist-grid { max-width: var(--app-content-max-width); margin: 0 auto; padding: 1rem; display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 2rem; }
.empty-state { display: flex; flex-direction: column; align-items: center; padding: 4rem; gap: 1rem; color: var(--app-text-secondary); }
.create-form { padding: 1rem 0; }
</style>
