import type { CoverCandidate, PictureSlot } from '@/types/metadata'

// The curated, ordered registry of attached-picture types the editor offers.
// IDs are the canonical TagLib pictureType strings as stored in the files.
// Mirrors internal/metadataedit/picturetypes.go — keep the two in sync.
export interface PictureTypeDef {
    id: string
    label: string
    // The matching Cover Art Archive image type, for sorting search results.
    caaType?: string
}

export const PICTURE_TYPES: readonly PictureTypeDef[] = [
    { id: 'Front Cover', label: 'Front cover', caaType: 'Front' },
    { id: 'Back Cover', label: 'Back cover', caaType: 'Back' },
    { id: 'Media', label: 'Media (disc)', caaType: 'Medium' },
    { id: 'Leaflet Page', label: 'Booklet', caaType: 'Booklet' },
    { id: 'Artist', label: 'Artist' },
    { id: 'Band', label: 'Band' },
    { id: 'Illustration', label: 'Illustration', caaType: 'Other' },
    { id: 'Other', label: 'Other', caaType: 'Other' }
] as const

export function pictureTypeLabel(id: string): string {
    return PICTURE_TYPES.find((t) => t.id === id)?.label ?? id
}

// The editor's slot display/preference order (embedded first) and labels. Both
// slots are on disk; this is NOT the app's serving precedence, which still
// prefers an uploaded cover in aether's store over the folder file.
export const PICTURE_SLOTS: readonly PictureSlot[] = ['embedded', 'folder'] as const

export const PICTURE_SLOT_LABELS: Record<PictureSlot, string> = {
    embedded: 'embedded in file',
    folder: 'album folder'
}

// candidateMatchesType reports whether a Cover Art Archive candidate depicts
// the requested picture type.
export function candidateMatchesType(candidate: CoverCandidate, typeId: string): boolean {
    const caa = PICTURE_TYPES.find((t) => t.id === typeId)?.caaType
    if (!caa) return false
    return candidate.types?.includes(caa) ?? false
}

// sortCandidatesForType orders CAA results for a picker scoped to one type:
// type matches first, then (for the front cover) isFront images, rest after,
// preserving the archive's order within each group.
export function sortCandidatesForType(
    candidates: CoverCandidate[],
    typeId: string
): CoverCandidate[] {
    const rank = (c: CoverCandidate): number => {
        if (candidateMatchesType(c, typeId)) return 0
        if (typeId === 'Front Cover' && c.isFront) return 1
        return 2
    }
    return [...candidates].sort((a, b) => rank(a) - rank(b))
}
