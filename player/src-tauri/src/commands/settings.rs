use crate::credentials;
use crate::error::AppError;
use crate::settings::{self, SessionKind, Settings};
use crate::state::{connection_for, AppState};
use aether_core::subsonic::normalize_server_url;
use tauri::{AppHandle, State};

#[tauri::command]
pub fn get_settings(state: State<'_, AppState>) -> Settings {
    state.settings.read().unwrap().clone()
}

/// Remembers the server the user picked, in its normalised form (so
/// `http://h:8075/` is saved and returned as `http://h:8075`), and how the SPA
/// signs in to it. A stored key belongs to the server that issued it, so it is
/// deleted — unless the same server is saved again for a key session, which is
/// what Connect does after an outage: that never costs the key.
#[tauri::command]
pub fn set_server_url(
    app: AppHandle,
    state: State<'_, AppState>,
    url: String,
    session_kind: SessionKind,
) -> Result<Settings, AppError> {
    let server_url = normalize_server_url(&url)?;
    let mut current = state.settings.write().unwrap();
    let next = Settings {
        server_url: Some(server_url),
        session_kind,
    };
    let keep_key = settings::keeps_key(&current, &next);
    replace_settings(&app, &state, &mut current, next, keep_key)
}

/// Switches how the SPA signs in to the saved server (the login view's "Use an
/// API key instead" / "Sign in with a password instead"). Leaving the key
/// session deletes the stored key: a secret nothing uses must not stay on disk.
#[tauri::command]
pub fn set_session_kind(
    app: AppHandle,
    state: State<'_, AppState>,
    session_kind: SessionKind,
) -> Result<Settings, AppError> {
    let mut current = state.settings.write().unwrap();
    let next = Settings {
        server_url: current.server_url.clone(),
        session_kind,
    };
    let keep_key = settings::keeps_key(&current, &next);
    replace_settings(&app, &state, &mut current, next, keep_key)
}

/// Forgets the server, the sign-in kind and any stored key (Change server).
#[tauri::command]
pub fn clear_server_url(app: AppHandle, state: State<'_, AppState>) -> Result<Settings, AppError> {
    let mut current = state.settings.write().unwrap();
    let next = Settings::default();
    let keep_key = settings::keeps_key(&current, &next);
    replace_settings(&app, &state, &mut current, next, keep_key)
}

/// Persists `next` and makes it the current settings. The stored key is deleted
/// unless `keep_key`, and the connection is rebuilt from what is left: a key
/// session that kept its key stays connected, everything else disconnects (a
/// session belongs to the server and the sign-in it was made for). The key is
/// deleted before the settings are written, so a failure in between errs toward
/// less access. When a write fails, memory stays as it was, but the key file
/// may already be gone.
///
/// `current` is the settings write lock the caller already holds, and the key
/// lock is taken here, after it (the lock order on `AppState`), so `set_session`
/// and `set_api_key` never store a client for a server that is no longer the
/// configured one.
fn replace_settings(
    app: &AppHandle,
    state: &AppState,
    current: &mut Settings,
    next: Settings,
    keep_key: bool,
) -> Result<Settings, AppError> {
    let mut stored_key = state.api_key.write().unwrap();
    if !keep_key {
        credentials::clear(app).map_err(|e| AppError::internal(e.to_string()))?;
    }
    settings::save(app, &next).map_err(|e| AppError::internal(e.to_string()))?;
    if !keep_key {
        *stored_key = None;
    }
    *current = next.clone();
    *state.connection.write().unwrap() = connection_for(&next, stored_key.as_deref());
    Ok(next)
}
