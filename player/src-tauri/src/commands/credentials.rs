use crate::error::AppError;
use crate::settings::Settings;
use crate::state::AppState;
use tauri::State;

/// The stored API key, for the SPA to install on its own client at startup.
/// Infallible: null when none is stored.
#[tauri::command]
pub fn get_api_key(state: State<'_, AppState>) -> Option<String> {
    state.session.api_key()
}

// The two commands below write files, so they are async: Tauri runs a sync
// command on the main thread, where the disk would hold up the window.

/// Stores the key the SPA verified against the saved server and connects the
/// Rust client with it (see `Session::set_api_key`).
#[tauri::command]
pub async fn set_api_key(
    state: State<'_, AppState>,
    api_key: String,
) -> Result<Settings, AppError> {
    state.session.set_api_key(api_key)
}

/// Forgets the stored key — a logout, or a key the server stopped accepting —
/// and drops the connection; the session kind stays.
#[tauri::command]
pub async fn clear_api_key(state: State<'_, AppState>) -> Result<(), AppError> {
    state.session.clear_api_key()
}
