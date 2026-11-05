<script setup lang="ts">
import { computed } from 'vue'
import type { Grouping } from '@/types/subsonic'
import ListCard from '@/components/library/ListCard.vue'
import { subsonicClient } from '@/lib/api/subsonic'
import { versionedCoverUrl } from '@/composables/useCoverVersion'

const props = defineProps<{ grouping?: Grouping }>()

const coverUrl = computed(() => {
    const art = props.grouping?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return versionedCoverUrl(subsonicClient.getCoverArtUrl(art, 200), art)
})

const meta = computed(() => {
    const n = props.grouping?.albumCount
    return n === undefined ? [] : [`${n} ${n === 1 ? 'album' : 'albums'}`]
})
</script>

<template>
    <ListCard v-if="!grouping" class="grouping-row" />
    <ListCard
        v-else
        class="grouping-row"
        :to="{ name: 'grouping', params: { id: grouping.id } }"
        :title="grouping.name"
        :meta="meta"
        :cover-url="coverUrl"
        placeholder-icon="ms-collections-bookmark"
    />
</template>
