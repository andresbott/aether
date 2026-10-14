use crate::error::AppError;
use crate::settings::{SessionKind, Settings};
use crate::state::AppState;
use tauri::State;

#[tauri::command]
pub fn get_settings(state: State<'_, AppState>) -> Settings {
    state.session.settings()
}

// The commands below write files, so they are async: Tauri runs a sync command
// on the main thread, where the disk would hold up the window.

/// Saves the server the user picked, normalised, and how the SPA signs in to it
/// (see `Session::set_server_url` for what happens to a stored key).
#[tauri::command]
pub async fn set_server_url(
    state: State<'_, AppState>,
    url: String,
    session_kind: SessionKind,
) -> Result<Settings, AppError> {
    state.session.set_server_url(&url, session_kind)
}

/// Switches how the SPA signs in to the saved server.
#[tauri::command]
pub async fn set_session_kind(
    state: State<'_, AppState>,
    session_kind: SessionKind,
) -> Result<Settings, AppError> {
    state.session.set_session_kind(session_kind)
}

/// Forgets the server, the sign-in kind and any stored key (Change server).
#[tauri::command]
pub async fn clear_server_url(state: State<'_, AppState>) -> Result<Settings, AppError> {
    state.session.clear_server_url()
}
