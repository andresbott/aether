<script setup lang="ts">
import type { Playlist } from '@/types/subsonic'
import PlaylistRow from '@/components/library/PlaylistRow.vue'

// Row rendering (cover, title, star, counts) lives in PlaylistRow so this
// view and the Discovery feed share one playlist row. This view only adds the
// column header and the scroll container.
defineProps<{ playlists: Playlist[] }>()
</script>

<template>
    <div class="playlist-list content-col">
        <!-- Columns must mirror PlaylistRow's grid template. -->
        <div class="list-header">
            <span class="col-cover"></span>
            <span class="col-name">Playlist</span>
            <!-- Empty: stands in for AlbumRow's artist column, which a playlist has
                 no value for, and for the hover-revealed favorite column. -->
            <span class="col-artist"></span>
            <span class="col-star"></span>
            <span class="col-count">Songs</span>
            <span class="col-duration">Length</span>
        </div>
        <PlaylistRow v-for="pl in playlists" :key="pl.id" :playlist="pl" />
    </div>
</template>

<style scoped>
/* Row styling lives in PlaylistRow; only the header and container remain here.
   The header's columns mirror PlaylistRow's grid template so they line up. */
.playlist-list { padding-top: 0; padding-bottom: 0; }
.list-header {
    display: grid;
    grid-template-columns: 48px 2fr 1.5fr 2rem 4rem 5rem;
    align-items: center;
    gap: 1rem;
    padding: 0 0.5rem;
    height: 36px;
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
    border-bottom: 1px solid var(--p-content-border-color);
}
.list-header .col-count,
.list-header .col-duration { text-align: right; }
</style>
