/**
 * Viewport breakpoints, in px. SCSS twins live in assets/scss/_variables.scss
 * ($bp-phone-max / $bp-desktop-min); assets/scss/__tests__/breakpoints.spec.ts
 * asserts the two stay equal. See
 * docs/superpowers/specs/2026-08-13-mobile-responsive-design.md §2.1.
 *
 * - width <  BP_PHONE_MAX                 → phone  (mobile shell)
 * - BP_PHONE_MAX ≤ width < BP_DESKTOP_MIN → tablet (shell follows orientation)
 * - width ≥ BP_DESKTOP_MIN                → desktop (desktop shell)
 */
export const BP_PHONE_MAX = 768
export const BP_DESKTOP_MIN = 1024

/**
 * Minimum viewport height for the desktop shell in the tablet width band.
 * Landscape PHONES land in that band too (iPhone 15: 852x393, Pixel 8:
 * 915x412) and must stay on the mobile shell; a landscape TABLET is at least
 * this tall. JS-only — the shell decision lives in useViewport, not CSS.
 */
export const BP_SHELL_MIN_HEIGHT = 600
