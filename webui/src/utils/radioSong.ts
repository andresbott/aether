import type { InternetRadioStation, Song } from '@/types/subsonic'

/**
 * Adapt an internet radio station into the `Song` shape the player and queue
 * understand. Stations have no track id of their own, so a stable synthetic id
 * is derived from the name. Used for both click-to-play and drag-to-queue.
 */
export function stationToSong(station: InternetRadioStation): Song {
    return {
        id: `radio-${station.name}`,
        title: station.name,
        artist: 'Internet Radio',
        streamUrl: station.streamUrl,
        coverArt: station.coverArt
    }
}
