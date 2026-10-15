// What makes an answer Aether's /me (server/app/router/handlers/me.go): the
// gate decides on it, so a 200 from something else at the same address — another
// web app on that port, a captive portal's page, a proxy's default page — must
// never pass for it. Shared by the bootstrap (getMe) and the desktop app's server
// probe, so both read an answer the same way.
import type { AuthMethod, MeResponse } from '@/types/users'

const AUTH_METHODS: readonly AuthMethod[] = ['none', 'native', 'proxy-header']

/**
 * The server answered /api/v0/me, but not as Aether does. getMe rejects with it,
 * so the desktop app's server step can tell "something else answered" from "no
 * answer".
 */
export class NotAetherMeError extends Error {
    name = 'NotAetherMeError'

    constructor() {
        super('The server did not answer /api/v0/me as an Aether server.')
    }
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null && !Array.isArray(value)
}

/**
 * True for an answer shaped like Aether's /me: one of the three auth methods, a
 * user that is null or has a login, and a features object. Aether always sends
 * all three fields.
 */
export function isMeResponse(value: unknown): value is MeResponse {
    if (!isRecord(value)) return false
    const { authMethod, user, features } = value
    return (
        AUTH_METHODS.includes(authMethod as AuthMethod) &&
        (user === null || (isRecord(user) && typeof user.login === 'string')) &&
        isRecord(features)
    )
}
