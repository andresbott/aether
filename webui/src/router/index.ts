import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
    {
        path: '/',
        name: 'home',
        component: () => import('@/views/HomeView.vue')
    },
    {
        path: '/library',
        name: 'library',
        component: () => import('@/views/LibraryView.vue')
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
        props: true
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
        component: () => import('@/views/RadioView.vue')
    },
    {
        path: '/song/:index',
        name: 'song',
        component: () => import('@/views/SongView.vue'),
        props: true
    },
    {
        path: '/admin',
        name: 'admin',
        component: () => import('@/views/AdminView.vue')
    },
    {
        path: '/settings',
        name: 'settings',
        component: () => import('@/views/SettingsView.vue')
    }
]

const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    routes
})

export default router
