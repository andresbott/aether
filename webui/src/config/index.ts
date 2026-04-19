export interface AppConfig {
    subsonic: {
        serverUrl: string
        username: string
        password: string
    }
}

export const config: AppConfig = {
    subsonic: {
        serverUrl: import.meta.env.VITE_SUBSONIC_SERVER_URL || '',
        username: import.meta.env.VITE_SUBSONIC_USERNAME || '',
        password: import.meta.env.VITE_SUBSONIC_PASSWORD || ''
    }
}

export function hasCredentials(): boolean {
    return !!(config.subsonic.serverUrl && config.subsonic.username && config.subsonic.password)
}
