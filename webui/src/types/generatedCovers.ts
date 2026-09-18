export interface CoverStyleInfo {
    name: string
    label: string
}

export interface GeneratedCoverSettings {
    styles: CoverStyleInfo[]
    default: string[]
    available: string[]
}

export interface GeneratedCoverSettingsInput {
    default: string[]
    available: string[]
}
