# Aether

Music server written in Go with a Vue 3 frontend.

## Project State

- **No backwards compatibility — until further notice.** No users, no live deployment. Do not write migration code, schema bridges, config compat layers, or "if old shape, fall back to..." branches. When the schema or config shape changes, just change it; the user will drop the DB manually if needed. Structure can change freely.
- SQLite + GORM for persistence, gorilla/mux for routing, PrimeVue for UI.
- Single binary deployment with embedded SPA.
