import type {
    SubsonicResponse,
    SubsonicCredentials,
    SearchParams,
    SearchResult3,
    Album,
    AlbumWithSongs,
    AlbumIndex,
    AlbumLetter,
    Artist,
    ArtistIndex,
    Song,
    Playlist,
    MusicFolder,
    Genre,
    InternetRadioStation,
    DiscoveryPage,
    SavedPlayQueue
} from '@/types/subsonic'

class SubsonicClient {
    private credentials: SubsonicCredentials | null = null
    private serverUrl: string = ''
    private authSkipped: boolean = false
    private readonly clientName = 'aether-web'
    private readonly apiVersion = '1.16.1'

    initWithDefaults(): void {
        this.serverUrl = window.location.origin
        this.authSkipped = true
    }

    setCredentials(credentials: SubsonicCredentials): void {
        this.credentials = credentials
        this.serverUrl = credentials.serverUrl
        this.authSkipped = false
    }

    getCredentials(): SubsonicCredentials | null {
        return this.credentials
    }

    isConfigured(): boolean {
        return this.authSkipped || this.credentials !== null
    }

    private buildUrl(
        endpoint: string,
        params: Record<string, string | number | boolean | undefined> = {}
    ): string {
        if (!this.isConfigured()) {
            throw new Error('Credentials not set')
        }

        const url = new URL(`${this.serverUrl}/rest/${endpoint}`)

        if (this.credentials) {
            url.searchParams.append('u', this.credentials.username)
            if (this.credentials.token && this.credentials.salt) {
                url.searchParams.append('t', this.credentials.token)
                url.searchParams.append('s', this.credentials.salt)
            } else if (this.credentials.password) {
                url.searchParams.append('p', this.credentials.password)
            }
        }

        url.searchParams.append('v', this.apiVersion)
        url.searchParams.append('c', this.clientName)
        url.searchParams.append('f', 'json')

        Object.entries(params).forEach(([key, value]) => {
            if (value !== undefined) {
                url.searchParams.append(key, String(value))
            }
        })

        return url.toString()
    }

    private async request<T>(
        endpoint: string,
        params: Record<string, string | number | boolean | undefined> = {}
    ): Promise<T> {
        const url = this.buildUrl(endpoint, params)

        const response = await fetch(url)
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`)
        }

        const data = (await response.json()) as SubsonicResponse<T>

        if (data['subsonic-response'].status === 'failed') {
            throw new Error(data['subsonic-response'].error?.message || 'Unknown error')
        }

        return data['subsonic-response'] as T
    }

    async ping(): Promise<boolean> {
        if (!this.isConfigured()) return false
        try {
            await this.request('ping.view')
            return true
        } catch {
            return false
        }
    }

    async getMusicFolders(): Promise<MusicFolder[]> {
        if (!this.isConfigured()) return []
        const response = await this.request<{
            musicFolders: { musicFolder: MusicFolder[] }
        }>('getMusicFolders.view')
        return response.musicFolders?.musicFolder || []
    }

    async getAlbumList(
        type: string,
        size = 20,
        offset = 0,
        musicFolderId?: number
    ): Promise<Album[]> {
        if (!this.isConfigured()) return []
        const params: Record<string, string | number | undefined> = { type, size, offset }
        if (musicFolderId !== undefined) {
            params.musicFolderId = musicFolderId
        }
        const response = await this.request<{ albumList2: { album: Album[] } }>(
            'getAlbumList2.view',
            params
        )
        return response.albumList2?.album || []
    }

    async getAlbumIndex(musicFolderId?: number): Promise<AlbumIndex> {
        if (!this.isConfigured()) return { total: 0, index: [] }
        const params: Record<string, string | number | undefined> = {}
        if (musicFolderId !== undefined) {
            params.musicFolderId = musicFolderId
        }
        const response = await this.request<{ albumList2Index?: AlbumIndex }>(
            'getAlbumList2Index.view',
            params
        )
        return response.albumList2Index ?? { total: 0, index: [] }
    }

    async getDiscovery(
        size: number,
        offset: number,
        seed: number,
        musicFolderId?: number
    ): Promise<DiscoveryPage> {
        if (!this.isConfigured()) return { album: [], playlist: [] }
        const params: Record<string, string | number | undefined> = { size, offset, seed }
        if (musicFolderId !== undefined) {
            params.musicFolderId = musicFolderId
        }
        const response = await this.request<{ discovery?: DiscoveryPage }>(
            'getDiscovery.view',
            params
        )
        return {
            album: response.discovery?.album ?? [],
            playlist: response.discovery?.playlist ?? []
        }
    }

    async getAlbum(id: string): Promise<AlbumWithSongs | null> {
        if (!this.isConfigured()) return null
        const response = await this.request<{ album: AlbumWithSongs }>('getAlbum.view', { id })
        return response.album
    }

    async getArtist(id: string): Promise<(Artist & { album?: Album[] }) | null> {
        if (!this.isConfigured()) return null
        const response = await this.request<{ artist: Artist & { album?: Album[] } }>(
            'getArtist.view',
            { id }
        )
        return response.artist
    }

    async getArtistIndex(musicFolderId?: number): Promise<ArtistIndex> {
        if (!this.isConfigured()) return { total: 0, letters: [], items: [] }
        const params: Record<string, string | number | undefined> = {}
        if (musicFolderId !== undefined) {
            params.musicFolderId = musicFolderId
        }
        const response = await this.request<{
            artists?: { index?: Array<{ name: string; artist?: Artist[] }> }
        }>('getArtists.view', params)

        const letters: AlbumLetter[] = []
        const items: Artist[] = []
        for (const group of response.artists?.index ?? []) {
            const groupArtists = group.artist ?? []
            if (groupArtists.length === 0) continue
            letters.push({ name: group.name, offset: items.length, count: groupArtists.length })
            items.push(...groupArtists)
        }
        return { total: items.length, letters, items }
    }

    async getArtists(musicFolderId?: number): Promise<Artist[]> {
        return (await this.getArtistIndex(musicFolderId)).items
    }

    async getGenres(): Promise<Genre[]> {
        if (!this.isConfigured()) return []
        const response = await this.request<{ genres: { genre: Genre[] } }>('getGenres.view')
        return response.genres?.genre || []
    }

    async getSongsByGenre(genre: string, count = 100, offset = 0): Promise<Song[]> {
        if (!this.isConfigured()) return []
        const response = await this.request<{ songsByGenre: { song: Song[] } }>(
            'getSongsByGenre.view',
            { genre, count, offset }
        )
        return response.songsByGenre?.song || []
    }

    async updateGenreCover(
        genreId: string,
        coverFile?: File,
        coverClear?: boolean
    ): Promise<void> {
        if (!this.isConfigured()) return
        const url = this.buildUrl('updateGenre.view')
        const body = new FormData()
        body.append('id', genreId)
        if (coverFile) body.append('coverFile', coverFile)
        if (coverClear) body.append('coverClear', 'true')
        await this.submitMultipart(url, body)
    }

    async search(params: SearchParams): Promise<SearchResult3> {
        if (!this.isConfigured()) return {}
        const response = await this.request<{ searchResult3: SearchResult3 }>(
            'search3.view',
            params as unknown as Record<string, string | number>
        )
        return response.searchResult3
    }

    async getPlaylists(): Promise<Playlist[]> {
        if (!this.isConfigured()) return []
        const response = await this.request<{ playlists: { playlist: Playlist[] } }>(
            'getPlaylists.view'
        )
        return response.playlists?.playlist || []
    }

    async getPlaylist(id: string): Promise<(Playlist & { entry?: Song[] }) | null> {
        if (!this.isConfigured()) return null
        const response = await this.request<{ playlist: Playlist & { entry?: Song[] } }>(
            'getPlaylist.view',
            { id }
        )
        return response.playlist
    }

    async getInternetRadioStations(): Promise<InternetRadioStation[]> {
        if (!this.isConfigured()) return []
        const response = await this.request<{
            internetRadioStations: { internetRadioStation: InternetRadioStation[] }
        }>('getInternetRadioStations.view')
        return response.internetRadioStations?.internetRadioStation || []
    }

    async createInternetRadioStation(
        name: string,
        streamUrl: string,
        homepageUrl?: string,
        coverFile?: File
    ): Promise<void> {
        if (!this.isConfigured()) return
        const url = this.buildUrl('createInternetRadioStation.view')
        const body = new FormData()
        body.append('name', name)
        body.append('streamUrl', streamUrl)
        if (homepageUrl) body.append('homepageUrl', homepageUrl)
        if (coverFile) body.append('coverFile', coverFile)
        await this.submitMultipart(url, body)
    }

    async updateInternetRadioStation(
        id: string,
        name: string,
        streamUrl: string,
        homepageUrl?: string,
        coverFile?: File,
        coverClear?: boolean
    ): Promise<void> {
        if (!this.isConfigured()) return
        const url = this.buildUrl('updateInternetRadioStation.view')
        const body = new FormData()
        body.append('id', id)
        body.append('name', name)
        body.append('streamUrl', streamUrl)
        if (homepageUrl) body.append('homepageUrl', homepageUrl)
        if (coverFile) body.append('coverFile', coverFile)
        if (coverClear) body.append('coverClear', 'true')
        await this.submitMultipart(url, body)
    }

    private async submitMultipart(url: string, body: FormData): Promise<void> {
        const response = await fetch(url, { method: 'POST', body })
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`)
        const data = await response.json()
        if (data['subsonic-response'].status === 'failed') {
            throw new Error(data['subsonic-response'].error?.message || 'Unknown error')
        }
    }

    async deleteInternetRadioStation(id: string): Promise<void> {
        if (!this.isConfigured()) return
        await this.request('deleteInternetRadioStation.view', { id })
    }

    getStreamUrl(id: string, maxBitRate?: number): string {
        const params: Record<string, string | number | undefined> = { id }
        if (maxBitRate) {
            params.maxBitRate = maxBitRate
        }
        return this.buildUrl('stream.view', params)
    }

    getCoverArtUrl(id: string, size?: number): string {
        const params: Record<string, string | number | undefined> = { id }
        if (size) {
            params.size = size
        }
        return this.buildUrl('getCoverArt.view', params)
    }

    async createPlaylist(name: string, songIds?: string[]): Promise<Playlist | null> {
        if (!this.isConfigured()) return null
        const params: Record<string, string | number | undefined> = { name }
        const url = new URL(this.buildUrl('createPlaylist.view', params))
        if (songIds) {
            songIds.forEach(id => url.searchParams.append('songId', id))
        }
        const response = await fetch(url.toString())
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`)
        const data = (await response.json()) as SubsonicResponse<{ playlist: Playlist }>
        if (data['subsonic-response'].status === 'failed') {
            throw new Error(data['subsonic-response'].error?.message || 'Unknown error')
        }
        return data['subsonic-response'].playlist
    }

    async updatePlaylist(
        playlistId: string,
        options: { name?: string; comment?: string; songIdsToAdd?: string[]; songIndexesToRemove?: number[] }
    ): Promise<void> {
        if (!this.isConfigured()) return
        const params: Record<string, string | number | undefined> = { playlistId }
        if (options.name) params.name = options.name
        if (options.comment) params.comment = options.comment
        const url = new URL(this.buildUrl('updatePlaylist.view', params))
        if (options.songIdsToAdd) {
            options.songIdsToAdd.forEach(id => url.searchParams.append('songIdToAdd', id))
        }
        if (options.songIndexesToRemove) {
            options.songIndexesToRemove.forEach(idx => url.searchParams.append('songIndexToRemove', String(idx)))
        }
        const response = await fetch(url.toString())
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`)
        const data = await response.json()
        if (data['subsonic-response'].status === 'failed') {
            throw new Error(data['subsonic-response'].error?.message || 'Unknown error')
        }
    }

    async updatePlaylistCover(
        playlistId: string,
        coverFile?: File,
        coverClear?: boolean
    ): Promise<void> {
        if (!this.isConfigured()) return
        const url = this.buildUrl('updatePlaylist.view')
        const body = new FormData()
        body.append('playlistId', playlistId)
        if (coverFile) body.append('coverFile', coverFile)
        if (coverClear) body.append('coverClear', 'true')
        await this.submitMultipart(url, body)
    }

    async updateArtistCover(
        artistId: string,
        coverFile?: File,
        coverClear?: boolean
    ): Promise<void> {
        if (!this.isConfigured()) return
        const url = this.buildUrl('updateArtist.view')
        const body = new FormData()
        body.append('id', artistId)
        if (coverFile) body.append('coverFile', coverFile)
        if (coverClear) body.append('coverClear', 'true')
        await this.submitMultipart(url, body)
    }

    async replacePlaylistTracks(playlistId: string, songIds: string[]): Promise<void> {
        if (!this.isConfigured()) return
        const url = new URL(this.buildUrl('createPlaylist.view', { playlistId }))
        songIds.forEach((id) => url.searchParams.append('songId', id))
        const response = await fetch(url.toString())
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`)
        const data = await response.json()
        if (data['subsonic-response'].status === 'failed') {
            throw new Error(data['subsonic-response'].error?.message || 'Unknown error')
        }
    }

    async deletePlaylist(id: string): Promise<void> {
        if (!this.isConfigured()) return
        await this.request('deletePlaylist.view', { id })
    }

    async star(id: string): Promise<void> {
        if (!this.isConfigured()) return
        await this.request('star.view', { id })
    }

    async unstar(id: string): Promise<void> {
        if (!this.isConfigured()) return
        await this.request('unstar.view', { id })
    }

    // Fire-and-forget: a failed scrobble must never interrupt playback, so this
    // swallows every error instead of propagating it to the caller.
    async scrobble(id: string): Promise<void> {
        if (!this.isConfigured()) return
        try {
            await this.request('scrobble.view', { id, submission: true })
        } catch (err) {
            console.warn('scrobble failed', err)
        }
    }

    // Persists the queue so another browser or device can resume the session.
    // Uses the index-based variant ("indexBasedQueue" extension) rather than the
    // id-based one: a queue may hold the same track twice, and `current` as a
    // track id could not say which copy is playing.
    //
    // An empty queue is the spec's clear call — no `id`, and `currentIndex` must
    // be omitted entirely or the server rejects it.
    //
    // Fire-and-forget, like scrobble: queue persistence is a background
    // convenience and must never break playback.
    async savePlayQueue(songIds: string[], currentIndex: number, positionMs: number): Promise<void> {
        if (!this.isConfigured()) return
        try {
            const url = new URL(this.buildUrl('savePlayQueueByIndex.view'))
            songIds.forEach((id) => url.searchParams.append('id', id))
            if (songIds.length > 0) {
                url.searchParams.append('currentIndex', String(currentIndex))
                url.searchParams.append('position', String(Math.max(0, Math.round(positionMs))))
            }
            const response = await fetch(url.toString())
            if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`)
            const data = await response.json()
            if (data['subsonic-response'].status === 'failed') {
                throw new Error(data['subsonic-response'].error?.message || 'Unknown error')
            }
        } catch (err) {
            console.warn('savePlayQueue failed', err)
        }
    }

    // The unload-safe variant of savePlayQueue. A normal fetch is cancelled when the
    // tab goes away, so the last write of a session has to leave as a beacon or the
    // position never lands. Returns whether the beacon was handed off.
    //
    // Never throws: this runs during unload, where an exception can hold up the page
    // closing and there is no UI left to report it to. An empty queue sends nothing —
    // it would clobber a queue saved from another device.
    savePlayQueueBeacon(songIds: string[], currentIndex: number, positionMs: number): boolean {
        if (!this.isConfigured()) return false
        if (songIds.length === 0) return false
        try {
            if (typeof navigator?.sendBeacon !== 'function') return false
            const url = new URL(this.buildUrl('savePlayQueueByIndex.view'))
            songIds.forEach((id) => url.searchParams.append('id', id))
            url.searchParams.append('currentIndex', String(currentIndex))
            url.searchParams.append('position', String(Math.max(0, Math.round(positionMs))))
            return navigator.sendBeacon(url.toString())
        } catch {
            return false
        }
    }

    // Returns null when nothing is saved (a fresh account) or the request fails —
    // both mean "no session to restore", and neither should block startup.
    async getPlayQueue(): Promise<SavedPlayQueue | null> {
        if (!this.isConfigured()) return null
        try {
            const response = await this.request<{ playQueueByIndex?: SavedPlayQueue }>(
                'getPlayQueueByIndex.view'
            )
            const queue = response.playQueueByIndex
            if (!queue || !queue.entry || queue.entry.length === 0) return null
            return queue
        } catch (err) {
            console.warn('getPlayQueue failed', err)
            return null
        }
    }

    async getRandomSongs(size = 50, musicFolderId?: number): Promise<Song[]> {
        if (!this.isConfigured()) return []
        const params: Record<string, string | number | undefined> = { size }
        if (musicFolderId !== undefined) {
            params.musicFolderId = musicFolderId
        }
        const response = await this.request<{ randomSongs: { song: Song[] } }>(
            'getRandomSongs.view',
            params
        )
        return response.randomSongs?.song || []
    }
}

export const subsonicClient = new SubsonicClient()
