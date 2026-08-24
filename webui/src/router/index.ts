import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
    {
        path: '/',
        name: 'home',
        component: () => import('@/views/HomeView.vue'),
        meta: { flush: true }
    },
    {
        // Mobile-only landing page and nav surface — the phone's stand-in for the
        // desktop sidebar, and where every top-level view's hamburger goes. The
        // view redirects to /library at desktop width (see MobileBrowseView).
        path: '/browse',
        name: 'browse',
        component: () => import('@/views/MobileBrowseView.vue'),
        meta: { flush: true }
    },
    {
        path: '/search',
        name: 'search',
        component: () => import('@/views/SearchView.vue'),
        meta: { flush: true }
    },
    {
        // The section is a path segment (/user-settings/access) so a reload or a
        // shared link lands on the same panel. Bare /user-settings is the
        // default (General) section; the view rewrites an unknown or
        // unavailable section back to it.
        path: '/user-settings/:tab?',
        name: 'user-settings',
        component: () => import('@/views/UserSettingsView.vue'),
        meta: { flush: true }
    },
    {
        path: '/about',
        name: 'about',
        component: () => import('@/views/AboutView.vue'),
        meta: { flush: true }
    },
    {
        path: '/library/:folderId?',
        name: 'library',
        component: () => import('@/views/LibraryView.vue'),
        props: true,
        meta: { flush: true }
    },
    {
        path: '/album/:id',
        name: 'album',
        component: () => import('@/views/AlbumView.vue'),
        props: true,
        meta: { flush: true }
    },
    {
        path: '/artist/:id',
        name: 'artist',
        component: () => import('@/views/ArtistView.vue'),
        props: true,
        meta: { flush: true }
    },
    {
        path: '/playlists',
        name: 'playlists',
        component: () => import('@/views/PlaylistsView.vue'),
        meta: { flush: true }
    },
    {
        path: '/playlist/:id',
        name: 'playlist-detail',
        component: () => import('@/views/PlaylistDetailView.vue'),
        props: true,
        meta: { flush: true }
    },
    {
        path: '/genres',
        name: 'genres',
        component: () => import('@/views/GenresView.vue'),
        meta: { flush: true }
    },
    {
        path: '/genre/:name',
        name: 'genre-detail',
        component: () => import('@/views/GenreDetailView.vue'),
        props: true,
        meta: { flush: true }
    },
    {
        path: '/radio',
        name: 'radio',
        component: () => import('@/views/RadioView.vue'),
        meta: { flush: true }
    },
    {
        path: '/radio/new',
        name: 'radio-station-new',
        component: () => import('@/views/RadioStationDetailView.vue'),
        props: { create: true },
        meta: { flush: true }
    },
    {
        path: '/radio/:id',
        name: 'radio-station-detail',
        component: () => import('@/views/RadioStationDetailView.vue'),
        props: true,
        meta: { flush: true }
    },
    {
        // A top-level route, but rendered in the admin-only settings layout so
        // the App.vue redirect (meta.layout === 'settings') still guards it. It
        // is reached from the sidebar UserMenu, not the Settings side-nav.
        path: '/metadata-editor',
        name: 'metadata-editor',
        component: () => import('@/views/settings/MetadataEditorView.vue'),
        meta: { layout: 'settings' }
    },
    {
        path: '/settings',
        component: () => import('@/views/settings/SettingsView.vue'),
        meta: { layout: 'settings' },
        children: [
            {
                path: '',
                name: 'settings',
                redirect: '/settings/libraries'
            },
            {
                path: 'libraries',
                name: 'settings-libraries',
                component: () => import('@/views/settings/LibrariesView.vue')
            },
            {
                path: 'users',
                name: 'settings-users',
                component: () => import('@/views/settings/UsersView.vue')
            },
            {
                path: 'tasks',
                name: 'settings-tasks',
                component: () => import('@/views/settings/TasksView.vue')
            }
        ]
    }
]

const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    routes
})

export default router
