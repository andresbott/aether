<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Menu from 'primevue/menu'
import type { MenuItem } from 'primevue/menuitem'
import BrandMark from '@/components/common/BrandMark.vue'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import BrowseAlbumShelf from '@/components/library/BrowseAlbumShelf.vue'
import BrowseShelf from '@/components/library/BrowseShelf.vue'
import DiscoveryFeedItem from '@/components/library/DiscoveryFeedItem.vue'
import GenreCard from '@/components/library/GenreCard.vue'
import PlaylistCard from '@/components/library/PlaylistCard.vue'
import RadioStationCard from '@/components/library/RadioStationCard.vue'
import { useAuth } from '@/composables/useAuth'
import { useDiscoveryFeed } from '@/composables/useDiscovery'
import {
    useGenres,
    useMusicFolders,
    usePlaylists,
    useRadioStations
} from '@/composables/useSubsonicQueries'
import { useViewport } from '@/composables/useViewport'
import { BROWSE_SHELF_SIZE } from '@/lib/browseShelf'

/**
 * The mobile shell's landing page AND its whole navigation surface — the phone's
 * answer to the desktop sidebar, and where the hamburger in every top-level
 * view's header goes (see `ContentScaffold`). It replaced `MobileNavDrawer`: a
 * page can show what is *in* each destination instead of only naming it, and it
 * needs none of an overlay's route-change / system-back bookkeeping, since
 * leaving it is ordinary navigation.
 *
 * Sections follow the sidebar's order — Library, one shelf per dynamic library,
 * Playlists, Genres, Radio — each a heading, a few items, and a "See all" link to
 * the full view; the last three drop out of the page entirely when they hold
 * nothing (see the show* computeds). Two things the drawer listed have no shelf
 * to fill and live in
 * the header instead: Search (a search box samples nothing) and the account
 * entries the desktop keeps in the `UserMenu` popup — User settings / Admin /
 * About, plus Log out, behind the ⋮. Now Playing and the queue stay reachable
 * through the `MiniPlayer`, which docks here as on any other route.
 *
 * Mobile only: the desktop shell has the sidebar, so arriving at desktop width
 * (bookmark, rotation) replaces the route with /library — the mirror of the guard
 * `HomeView` applies in the other direction.
 */
const router = useRouter()
const { shell } = useViewport()
const { authRequired, isAdmin, logout } = useAuth()

watch(
    shell,
    (currentShell) => {
        if (currentShell === 'desktop') void router.replace({ name: 'library' })
    },
    { immediate: true }
)

// The Library shelf samples the ranked Discovery feed — the same query
// `/library`'s Discover tab renders in full, so this costs no extra request once
// either surface has been visited. It is deliberately not library-scoped: the
// ranking is cross-collection (see useDiscovery).
const {
    items: discoveryItems,
    isLoading: discoveryLoading,
    isError: discoveryError
} = useDiscoveryFeed()
const discoveryShelf = computed(() =>
    discoveryItems.value.slice(0, BROWSE_SHELF_SIZE).map((entry) => ({
        key: entry.type === 'album' ? entry.album.id : entry.playlist.id,
        entry
    }))
)

// Same rule as the sidebar and the old drawer: a single library needs no section
// of its own, since the Library shelf above already covers everything in it.
const { data: musicFolders } = useMusicFolders()
const libraryShelves = computed(() => {
    const folders = musicFolders.value ?? []
    return folders.length > 1 ? folders : []
})

const { data: playlists, isError: playlistsError } = usePlaylists()
const playlistShelf = computed(() =>
    (playlists.value ?? [])
        .slice(0, BROWSE_SHELF_SIZE)
        .map((playlist) => ({ key: playlist.id, playlist }))
)

const { data: genres, isError: genresError } = useGenres()
// Keyed by `value`: a genre has no id (see BrowseShelf).
const genreShelf = computed(() =>
    (genres.value ?? []).slice(0, BROWSE_SHELF_SIZE).map((genre) => ({ key: genre.value, genre }))
)

const { data: stations, isError: stationsError } = useRadioStations()
const radioShelf = computed(() =>
    (stations.value ?? []).slice(0, BROWSE_SHELF_SIZE).map((station) => ({ key: station.id, station }))
)

// Playlists, Genres and Radio are left OUT of the page entirely when they hold
// nothing: a heading and a "See all" over empty space is chrome pointing at
// nothing, and the landing page should read as what this server actually has.
// Two consequences on purpose:
//   - a failed request still renders the shelf, since its error line is the one
//     thing an item-less shelf has to say — hiding it would report a network
//     failure as "you have no playlists";
//   - a section in flight stays out and appears with its items rather than
//     flashing a spinner that turns out to be nothing.
// The Library shelf is exempt: it is the page's primary destination, so an empty
// library says so rather than leaving the page blank.
const showPlaylists = computed(() => playlistShelf.value.length > 0 || playlistsError.value)
const showGenres = computed(() => genreShelf.value.length > 0 || genresError.value)
const showRadio = computed(() => radioShelf.value.length > 0 || stationsError.value)

// Same entries and order as UserMenu's popup, so the two account surfaces read
// alike: User settings → Admin (admins only) → About → Log out. Log out is gated
// on there being a session at all (auth method "none" has nothing to leave), and
// this is the phone's ONLY way out — the UserMenu lives in the desktop sidebar.
const accountMenu = ref<InstanceType<typeof Menu> | null>(null)
const accountItems = computed<MenuItem[]>(() => {
    const items: MenuItem[] = [
        {
            label: 'User settings',
            icon: 'pi pi-user',
            command: () => void router.push('/user-settings')
        }
    ]
    if (isAdmin.value) {
        items.push({ label: 'Admin', icon: 'pi pi-cog', command: () => void router.push('/settings') })
    }
    items.push({ label: 'About', icon: 'pi pi-info-circle', command: () => void router.push('/about') })
    if (authRequired.value) {
        items.push({ separator: true })
        items.push({ label: 'Log out', icon: 'pi pi-sign-out', command: () => logout.mutate() })
    }
    return items
})
const toggleAccountMenu = (event: Event): void => accountMenu.value?.toggle(event)
</script>

<template>
    <!-- nav-root: this view IS the hamburger's destination, so it carries no
         hamburger of its own. The scaffold's own `title` is empty because the
         brand takes its place — see #title-actions. -->
    <ContentScaffold class="browse-view" title="" nav-root>
        <!-- The brand IS this page's heading: /browse is the phone's home
             surface, so it carries the app's identity the way the desktop
             sidebar's does, rather than the word "Browse". It is the h1, so the
             page still has exactly one top-level heading. -->
        <template #title-actions>
            <h1 class="browse-brand">
                <BrandMark size="1.5rem" />
                <span>A<span class="browse-brand-accent">e</span>ther</span>
            </h1>
        </template>

        <template #actions>
            <Button
                class="browse-search-btn"
                icon="pi pi-search"
                text
                rounded
                aria-label="Search"
                @click="router.push({ name: 'search' })"
            />
            <Button
                class="browse-account-btn"
                icon="pi pi-ellipsis-v"
                text
                rounded
                aria-label="Settings and account"
                aria-haspopup="menu"
                @click="toggleAccountMenu"
            />
            <Menu ref="accountMenu" :model="accountItems" :popup="true" />
        </template>

        <div class="browse-body">
            <div class="browse-shelves content-col">
                <BrowseShelf
                    title="Library"
                    icon="pi pi-compass"
                    :to="{ name: 'library' }"
                    :items="discoveryShelf"
                    :loading="discoveryLoading"
                    :error="discoveryError"
                    error-text="Could not load the discovery feed"
                >
                    <template #card="{ item }">
                        <DiscoveryFeedItem :entry="item.entry" layout="grid" />
                    </template>
                </BrowseShelf>

                <BrowseAlbumShelf
                    v-for="folder in libraryShelves"
                    :key="folder.id"
                    :folder-id="folder.id"
                    :title="folder.name"
                    :icon="`pi pi-${folder.icon || 'folder'}`"
                />

                <BrowseShelf
                    v-if="showPlaylists"
                    title="Playlists"
                    icon="pi pi-list"
                    :to="{ name: 'playlists' }"
                    :items="playlistShelf"
                    :error="playlistsError"
                    error-text="Could not load playlists"
                >
                    <template #card="{ item }">
                        <PlaylistCard :playlist="item.playlist" />
                    </template>
                </BrowseShelf>

                <BrowseShelf
                    v-if="showGenres"
                    title="Genres"
                    icon="pi pi-tags"
                    :to="{ name: 'genres' }"
                    :items="genreShelf"
                    :error="genresError"
                    error-text="Could not load genres"
                >
                    <template #card="{ item }">
                        <GenreCard :genre="item.genre" />
                    </template>
                </BrowseShelf>

                <BrowseShelf
                    v-if="showRadio"
                    title="Radio"
                    icon="pi pi-wifi"
                    :to="{ name: 'radio' }"
                    :items="radioShelf"
                    :error="stationsError"
                    error-text="Could not load radio stations"
                >
                    <template #card="{ item }">
                        <RadioStationCard :station="item.station" />
                    </template>
                </BrowseShelf>
            </div>
        </div>
    </ContentScaffold>
</template>

<style scoped>
/* The scaffold header aligns its row on the text baseline, which this header has
   none of: the heading is an image plus a wordmark, and the actions are two round
   icon buttons. As a flex container the h1's baseline is its FIRST item's — the
   brand image's bottom edge — so baseline alignment lifts the whole brand above
   the buttons. Center the row (and the title box inside it) instead. */
.browse-view :deep(.scaffold-header-inner),
.browse-view :deep(.scaffold-title) {
    align-items: center;
}

/* Slotted into the scaffold's title slot, so ContentScaffold's own (scoped) h1
   rules never reach it — the heading owns its whole box here. Same wordmark
   weights as AppSidebar's brand, in the content palette rather than the nav
   one, since this header sits on the page background. */
.browse-brand {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    margin: 0;
    font-size: 1.35rem;
    font-weight: 800;
    letter-spacing: 0.02em;
    color: var(--app-text-primary);
    white-space: nowrap;
}

.browse-brand-accent {
    color: var(--app-accent);
}

/* Recipe B (docs/architecture/main-content-view-layout.md): the body scrolls
   itself and reserves the uniform rail clearance on the right, so the shelves sit
   on the same column as the scaffold header's title. --sb-w comes from
   PlayerLayout; never re-measure it here. */
.browse-body {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    /* With an empty queue there is no mini player below this page, so it is the
       bottom-most surface and reserves the home-indicator inset itself. */
    padding-bottom: calc(1rem + env(safe-area-inset-bottom));
    box-sizing: border-box;
}

.browse-shelves {
    display: flex;
    flex-direction: column;
}
</style>
