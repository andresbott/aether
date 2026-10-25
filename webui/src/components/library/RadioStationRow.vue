<script setup lang="ts">
import { computed } from 'vue'
import type { InternetRadioStation } from '@/types/subsonic'
import ListCard from '@/components/library/ListCard.vue'
import { subsonicClient } from '@/lib/api/subsonic'
import { useSongsDrag } from '@/composables/useSongsDrag'
import { stationToSong } from '@/utils/radioSong'
import { versionedCoverUrl } from '@/composables/useCoverVersion'

const props = defineProps<{ station?: InternetRadioStation }>()

const songsDrag = useSongsDrag()

const coverUrl = computed(() => {
    const art = props.station?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    const base = subsonicClient.getCoverArtUrl(art, 200)
    return versionedCoverUrl(base, art)
})

// The homepage is the only fact a station has: on the lighter details line, not
// in the bold of a key fact, as its grid card keeps it secondary.
const details = computed(() => (props.station?.homepageUrl ? [props.station.homepageUrl] : []))

const onDragStart = (event: DragEvent): void => {
    if (props.station) songsDrag.start(event, [stationToSong(props.station)], event.currentTarget as HTMLElement)
}
</script>

<template>
    <ListCard v-if="!station" class="radio-row" round />
    <ListCard
        v-else
        class="radio-row"
        :to="{ name: 'radio-station-detail', params: { id: station.id } }"
        :title="station.name"
        :details="details"
        :cover-url="coverUrl"
        placeholder-icon="ms-radio"
        round
        draggable="true"
        @dragstart="onDragStart"
        @dragend="songsDrag.end"
    />
</template>
