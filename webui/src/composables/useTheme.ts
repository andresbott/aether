import { ref, computed, watch, readonly } from 'vue'
import { loadFromLocalStorage, saveToLocalStorage } from '@/utils/localStorage'

/**
 * Theme preference. `auto` follows the OS `prefers-color-scheme`; `light`/`dark`
 * force a mode regardless of the OS. `winamp`/`crt` are hidden themes — see
 * HIDDEN_MODES. Not persisted — resets to `auto` on reload.
 */
export type ThemeMode = 'auto' | 'light' | 'dark' | 'winamp' | 'crt'

export interface ThemeOption {
    label: string
    value: ThemeMode
}

/**
 * Themes that stay out of the settings picker until unlocked. Both are dark
 * variants, so they ride on top of `.dark-mode` rather than replacing it.
 */
export const HIDDEN_MODES = ['winamp', 'crt'] as const
export type HiddenMode = (typeof HIDDEN_MODES)[number]

const STORAGE_KEY_UNLOCKED = 'aether:hiddenThemes'

export const BASE_THEME_OPTIONS: ThemeOption[] = [
    { label: 'Auto', value: 'auto' },
    { label: 'Light', value: 'light' },
    { label: 'Dark', value: 'dark' }
]

export const HIDDEN_THEME_OPTIONS: ThemeOption[] = [
    { label: 'Winamp', value: 'winamp' },
    { label: 'CRT', value: 'crt' }
]

function isHiddenMode(mode: ThemeMode): mode is HiddenMode {
    return (HIDDEN_MODES as readonly string[]).includes(mode)
}

// Module-level singleton state: one theme shared by the whole app.
const mode = ref<ThemeMode>('auto')
const systemPrefersDark = ref(false)
// Unlike `mode`, this one persists — an unlock earned once stays earned.
const hiddenUnlocked = ref(false)

const isDark = computed(
    () =>
        mode.value === 'dark' ||
        isHiddenMode(mode.value) ||
        (mode.value === 'auto' && systemPrefersDark.value)
)

const options = computed<ThemeOption[]>(() =>
    hiddenUnlocked.value ? [...BASE_THEME_OPTIONS, ...HIDDEN_THEME_OPTIONS] : BASE_THEME_OPTIONS
)

let initialized = false

/**
 * Wire the OS media-query listener and keep the theme classes on the root
 * element in sync with the current mode. Runs once; safe to call from anywhere.
 */
function initTheme(): void {
    if (initialized || typeof window === 'undefined') return
    initialized = true

    hiddenUnlocked.value = loadFromLocalStorage(STORAGE_KEY_UNLOCKED, false)

    const mql = window.matchMedia('(prefers-color-scheme: dark)')
    systemPrefersDark.value = mql.matches
    mql.addEventListener('change', (e) => {
        systemPrefersDark.value = e.matches
    })

    watch(
        isDark,
        (dark) => {
            document.documentElement.classList.toggle('dark-mode', dark)
        },
        { immediate: true }
    )

    // Hidden themes layer their palette over `.dark-mode`, so only one
    // `theme-*` class may be present at a time.
    watch(
        mode,
        (current) => {
            for (const hidden of HIDDEN_MODES) {
                document.documentElement.classList.toggle(
                    `theme-${hidden}`,
                    current === hidden
                )
            }
        },
        { immediate: true }
    )
}

/** Reveal the hidden themes in the settings picker, for good. */
function unlockHiddenThemes(): void {
    if (hiddenUnlocked.value) return
    hiddenUnlocked.value = true
    saveToLocalStorage(STORAGE_KEY_UNLOCKED, true)
}

/**
 * Switch to the next hidden theme, wrapping around. From a non-hidden mode this
 * lands on the first one.
 */
function cycleHiddenTheme(): ThemeOption {
    const current = HIDDEN_THEME_OPTIONS.findIndex((o) => o.value === mode.value)
    const next = HIDDEN_THEME_OPTIONS[(current + 1) % HIDDEN_THEME_OPTIONS.length]
    mode.value = next.value
    return next
}

export function useTheme() {
    initTheme()
    return {
        mode,
        isDark: readonly(isDark),
        options,
        hiddenUnlocked: readonly(hiddenUnlocked),
        unlockHiddenThemes,
        cycleHiddenTheme
    }
}
