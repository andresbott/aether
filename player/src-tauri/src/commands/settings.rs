use crate::error::AppError;
use crate::settings::{self, Settings};
use crate::state::AppState;
use aether_core::subsonic::normalize_server_url;
use tauri::{AppHandle, State};

#[tauri::command]
pub fn get_settings(state: State<'_, AppState>) -> Settings {
    state.settings.read().unwrap().clone()
}

/// Remembers the server the user picked, in its normalised form (so
/// `http://h:8075/` is saved and returned as `http://h:8075`).
#[tauri::command]
pub fn set_server_url(
    app: AppHandle,
    state: State<'_, AppState>,
    url: String,
) -> Result<Settings, AppError> {
    let server_url = normalize_server_url(&url)?;
    replace_settings(
        &app,
        &state,
        Settings {
            server_url: Some(server_url),
        },
    )
}

#[tauri::command]
pub fn clear_server_url(app: AppHandle, state: State<'_, AppState>) -> Result<Settings, AppError> {
    replace_settings(&app, &state, Settings::default())
}

/// Persists `next` and makes it the current settings. A session belongs to the
/// server it was minted on, so the connection is dropped with the old settings.
///
/// Nothing changes when the write fails. The settings stay write-locked
/// throughout (see the lock order on `AppState`), so `set_session` never
/// stores a client for a server that is no longer the configured one.
fn replace_settings(
    app: &AppHandle,
    state: &AppState,
    next: Settings,
) -> Result<Settings, AppError> {
    let mut current = state.settings.write().unwrap();
    settings::save(app, &next).map_err(|e| AppError::internal(e.to_string()))?;
    *current = next.clone();
    *state.connection.write().unwrap() = None;
    Ok(next)
}
