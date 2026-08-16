<script setup lang="ts">
import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import AlbumCard from '@/components/library/AlbumCard.vue'
import BrowseShelf from '@/components/library/BrowseShelf.vue'
import { queryKeys } from '@/composables/useSubsonicQueries'
import { subsonicClient } from '@/lib/api/subsonic'
import { BROWSE_SHELF_SIZE } from '@/lib/browseShelf'

/**
 * One dynamic library's newest albums, as a landing-page shelf.
 *
 * A component per library rather than a loop inside `MobileBrowseView`: each
 * shelf owns its own query, which is the only way to run one per folder —
 * composables cannot be called in a loop over a reactive list.
 *
 * The key is the shared `albumList` one, so this shelf and `/library`'s own
 * album queries hit the same cache entries.
 */
const props = defineProps<{ folderId: number; title: string; icon?: string }>()

const { data, isLoading, isError } = useQuery({
    queryKey: computed(() =>
        queryKeys.albumList('newest', BROWSE_SHELF_SIZE, 0, props.folderId)
    ),
    queryFn: () => subsonicClient.getAlbumList('newest', BROWSE_SHELF_SIZE, 0, props.folderId),
    staleTime: 5 * 60 * 1000
})

const items = computed(() => (data.value ?? []).map((album) => ({ key: album.id, album })))
</script>

<template>
    <BrowseShelf
        :title="title"
        :icon="icon"
        :to="{ name: 'library', params: { folderId: String(folderId) } }"
        :items="items"
        :loading="isLoading"
        :error="isError"
        error-text="Could not load this library"
        empty-text="No albums yet"
    >
        <template #card="{ item }">
            <AlbumCard :album="item.album" />
        </template>
    </BrowseShelf>
</template>
