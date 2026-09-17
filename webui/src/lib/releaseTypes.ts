// MusicBrainz release-type vocabulary shared by the metadata editor (primary +
// secondary controls) and the Releases browse tabs. A release group has exactly
// one primary type and zero-or-more secondary types.
export const PRIMARY_RELEASE_TYPES = ['Album', 'Single', 'EP', 'Broadcast', 'Other'] as const

export const SECONDARY_RELEASE_TYPES = [
    'Compilation',
    'Soundtrack',
    'Spokenword',
    'Interview',
    'Audiobook',
    'Audio drama',
    'Live',
    'Remix',
    'DJ-mix',
    'Mixtape/Street',
    'Demo',
    'Field recording'
] as const
