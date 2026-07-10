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
        path: '/search',
        name: 'search',
        component: () => import('@/views/SearchView.vue'),
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
        props: true
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
        component: () => import('@/views/PlaylistsView.vue')
    },
    {
        path: '/playlist/:id',
        name: 'playlist-detail',
        component: () => import('@/views/PlaylistDetailView.vue'),
        props: true
    },
    {
        path: '/genres',
        name: 'genres',
        component: () => import('@/views/GenresView.vue')
    },
    {
        path: '/podcasts',
        name: 'podcasts',
        component: () => import('@/views/PodcastsView.vue')
    },
    {
        path: '/podcast/:id',
        name: 'podcast-channel',
        component: () => import('@/views/PodcastChannelView.vue'),
        props: true
    },
    {
        path: '/radio',
        name: 'radio',
        component: () => import('@/views/RadioView.vue'),
        meta: { flush: true }
    },
    {
        path: '/admin',
        component: () => import('@/views/AdminView.vue'),
        meta: { layout: 'admin' },
        children: [
            {
                path: '',
                redirect: '/admin/libraries'
            },
            {
                path: 'libraries',
                name: 'admin-libraries',
                component: () => import('@/views/admin/AdminLibrariesView.vue'),
                meta: { layout: 'admin' }
            },
            {
                path: 'tasks',
                name: 'admin-tasks',
                component: () => import('@/views/admin/AdminTasksView.vue'),
                meta: { layout: 'admin' }
            },
            {
                path: 'metadata',
                name: 'admin-metadata',
                component: () => import('@/views/admin/MetadataEditorView.vue'),
                meta: { layout: 'admin' }
            }
        ]
    },
    {
        path: '/settings',
        component: () => import('@/views/settings/SettingsView.vue'),
        meta: { layout: 'settings' },
        children: [
            {
                path: '',
                redirect: '/settings/profile'
            },
            {
                path: 'profile',
                name: 'settings-profile',
                component: () => import('@/views/settings/ProfileView.vue')
            }
        ]
    }
]

const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    routes
})

export default router
