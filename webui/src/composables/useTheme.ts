import { ref, computed, watch, readonly } from 'vue'

/**
 * Theme preference. `auto` follows the OS `prefers-color-scheme`; `light`/`dark`
 * force a mode regardless of the OS. Not persisted — resets to `auto` on reload.
 */
export type ThemeMode = 'auto' | 'light' | 'dark'

export const THEME_OPTIONS: { label: string; value: ThemeMode }[] = [
    { label: 'Auto', value: 'auto' },
    { label: 'Light', value: 'light' },
    { label: 'Dark', value: 'dark' }
]

// Module-level singleton state: one theme shared by the whole app.
const mode = ref<ThemeMode>('auto')
const systemPrefersDark = ref(false)

const isDark = computed(
    () => mode.value === 'dark' || (mode.value === 'auto' && systemPrefersDark.value)
)

let initialized = false

/**
 * Wire the OS media-query listener and keep the `.dark-mode` class on the root
 * element in sync with `isDark`. Runs once; safe to call from anywhere.
 */
function initTheme(): void {
    if (initialized || typeof window === 'undefined') return
    initialized = true

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
}

export function useTheme() {
    initTheme()
    return {
        mode,
        isDark: readonly(isDark),
        options: THEME_OPTIONS
    }
}
