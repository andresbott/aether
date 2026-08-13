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
