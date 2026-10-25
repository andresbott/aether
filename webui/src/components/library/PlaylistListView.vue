<script setup lang="ts">
import type { Playlist } from '@/types/subsonic'
import PlaylistRow from '@/components/library/PlaylistRow.vue'

// Card rendering (cover, title, star, counts) lives in PlaylistRow so this view
// and the Discovery feed share one playlist card. This view only adds the column.
defineProps<{ playlists: Playlist[] }>()
</script>

<template>
    <div class="playlist-list content-col">
        <PlaylistRow v-for="pl in playlists" :key="pl.id" :playlist="pl" />
    </div>
</template>

<style scoped>
/* The same gaps as the grid layout, so switching layouts keeps the first item put. */
.playlist-list {
    padding-top: 1rem;
    padding-bottom: 1rem;
}
</style>
