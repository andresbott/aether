import type {
    SubsonicResponse,
    SubsonicCredentials,
    SearchParams,
    SearchResult3,
    Album,
    AlbumWithSongs,
    AlbumIndex,
    Artist,
    Song,
    Playlist,
    MusicFolder,
    PodcastChannel,
    InternetRadioStation
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

    async getArtists(musicFolderId?: number): Promise<Artist[]> {
        if (!this.isConfigured()) return []
        const params: Record<string, string | number | undefined> = {}
        if (musicFolderId !== undefined) {
            params.musicFolderId = musicFolderId
        }
        const response = await this.request<{
            artists: {
                index: Array<{
                    name: string
                    artist: Artist[]
                }>
            }
        }>('getArtists.view', params)

        const allArtists: Artist[] = []
        response.artists?.index?.forEach((index) => {
            if (index.artist) {
                allArtists.push(...index.artist)
            }
        })
        return allArtists
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

    async getPodcasts(includeEpisodes = true): Promise<PodcastChannel[]> {
        if (!this.isConfigured()) return []
        const response = await this.request<{ podcasts: { channel: PodcastChannel[] } }>(
            'getPodcasts.view',
            { includeEpisodes }
        )
        return response.podcasts?.channel || []
    }

    async getPodcastChannel(id: string): Promise<PodcastChannel | null> {
        if (!this.isConfigured()) return null
        const response = await this.request<{ podcasts: { channel: PodcastChannel[] } }>(
            'getPodcasts.view',
            { includeEpisodes: true, id }
        )
        if (response.podcasts?.channel && response.podcasts.channel.length > 0) {
            return response.podcasts.channel[0]
        }
        return null
    }

    async getNewestPodcasts(count = 20): Promise<PodcastChannel[]> {
        if (!this.isConfigured()) return []
        const response = await this.request<{
            newestPodcasts: { episode: PodcastChannel[] }
        }>('getNewestPodcasts.view', { count })
        return response.newestPodcasts?.episode || []
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
