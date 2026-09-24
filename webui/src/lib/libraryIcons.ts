// Library icons are Material Symbols names in Google's canonical snake_case
// ("queue_music") — the vocabulary the musicFolderIcon extension advertises.
// Kept free of import.meta.env: the Vite plugin in build/material-symbols
// imports it too, from Node.

export const DEFAULT_LIBRARY_ICON = 'folder'

// Where the build emits one SVG per catalogue icon, relative to the app base.
export const LIBRARY_ICON_DIR = 'icons/ms'

// Mirrors libraries.iconNameRe on the server.
export const LIBRARY_ICON_NAME = /^[a-z0-9]+(_[a-z0-9]+)*$/

// What the icon picker shows before anything is typed: the whole catalogue in
// alphabetical order opens on "10k", "10mp"… — these are the icons a music
// library is likely to want, grouped by theme. Every entry is checked against
// the catalogue; keep the list under MAX_ICON_RESULTS so it shows in full.
export const SUGGESTED_LIBRARY_ICONS: readonly string[] = [
    // music & audio
    'folder', 'folder_special', 'library_music', 'queue_music', 'album', 'music_note',
    'music_video', 'headphones', 'headset_mic', 'speaker', 'radio', 'podcasts',
    'mic', 'mic_external_on', 'piano', 'graphic_eq', 'equalizer', 'surround_sound',
    'artist', 'genres', 'lyrics', 'instant_mix', 'playlist_play', 'library_add_check',
    // moods & occasions
    'nightlife', 'celebration', 'party_mode', 'cake', 'festival', 'local_bar',
    'wine_bar', 'local_cafe', 'coffee', 'bedtime', 'dark_mode', 'light_mode',
    'sunny', 'nights_stay', 'spa', 'self_improvement', 'mood', 'sentiment_satisfied',
    'sentiment_calm', 'favorite', 'star', 'auto_awesome', 'electric_bolt', 'bolt',
    'whatshot', 'local_fire_department', 'ac_unit', 'water_drop',
    // nature & weather
    'eco', 'forest', 'park', 'beach_access', 'sailing', 'surfing',
    'hiking', 'landscape', 'rainy', 'thunderstorm',
    // activities
    'fitness_center', 'directions_run', 'sprint', 'pool', 'sports_esports', 'sports_soccer',
    'directions_car', 'commute', 'flight', 'train', 'directions_bike', 'menu_book',
    'school', 'work', 'home', 'cooking', 'restaurant',
    // film, TV & stage
    'theaters', 'movie', 'live_tv', 'tv', 'theater_comedy', 'stadium',
    // people & kids
    'child_care', 'toys', 'stroller', 'family_restroom', 'groups', 'face', 'psychology',
    // places & culture
    'public', 'language', 'church', 'temple_buddhist', 'mosque', 'castle',
    'location_city', 'travel_explore', 'flag',
    // collections & misc
    'history', 'schedule', 'trending_up', 'diamond', 'workspace_premium', 'military_tech',
    'rocket_launch', 'science', 'pets', 'all_inclusive', 'category', 'label',
    'bookmark', 'collections_bookmark', 'inventory_2', 'archive', 'cloud'
]

// The name to render: anything malformed (a hand-edited row, an obsolete name)
// falls back to the default rather than leaving a blank gap.
export function resolveLibraryIcon(name?: string | null): string {
    return name && LIBRARY_ICON_NAME.test(name) ? name : DEFAULT_LIBRARY_ICON
}
