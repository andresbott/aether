use crate::credentials;
use crate::error::AppError;
use crate::settings::{self, SessionKind, Settings};
use crate::state::AppState;
use aether_core::error::CoreError;
use aether_core::subsonic::SubsonicClient;
use tauri::{AppHandle, State};

/// The stored API key, for the SPA to install on its own client at startup.
/// Infallible: null when none is stored.
#[tauri::command]
pub fn get_api_key(state: State<'_, AppState>) -> Option<String> {
    state.api_key.read().unwrap().clone()
}

/// Stores the key the SPA verified against the saved server, makes this a key
/// session and connects the Rust client with it — no round trip, the SPA did
/// the verifying. `NotConnected` without a server, `InvalidServerUrl` for a
/// hand-edited address that no longer parses. When a file cannot be written,
/// memory stays as it was. The settings go first and the key second, so when
/// the key write fails `settings.json` may already say `apiKey` with no key
/// stored: a key session without a key, which only brings the key card back.
#[tauri::command]
pub fn set_api_key(
    app: AppHandle,
    state: State<'_, AppState>,
    api_key: String,
) -> Result<Settings, AppError> {
    // Lock order: settings → api_key → connection (see AppState).
    let mut current = state.settings.write().unwrap();
    let mut stored_key = state.api_key.write().unwrap();
    let server_url = current
        .server_url
        .as_deref()
        .ok_or(CoreError::NotConnected)?;
    let client = SubsonicClient::with_api_key(server_url, &api_key)?;
    let next = Settings {
        server_url: current.server_url.clone(),
        session_kind: SessionKind::ApiKey,
    };
    if *current != next {
        settings::save(&app, &next).map_err(|e| AppError::internal(e.to_string()))?;
    }
    credentials::save(&app, &api_key).map_err(|e| AppError::internal(e.to_string()))?;
    *current = next.clone();
    *stored_key = Some(api_key);
    *state.connection.write().unwrap() = Some(client);
    Ok(next)
}

/// Forgets the stored key — a logout, or a key the server stopped accepting —
/// and drops the connection. The session kind stays, so the key card comes
/// back rather than the server step.
#[tauri::command]
pub fn clear_api_key(app: AppHandle, state: State<'_, AppState>) -> Result<(), AppError> {
    // Never read: taken first only to keep the lock order settings → api_key → connection.
    let _settings = state.settings.read().unwrap();
    let mut stored_key = state.api_key.write().unwrap();
    credentials::clear(&app).map_err(|e| AppError::internal(e.to_string()))?;
    *stored_key = None;
    *state.connection.write().unwrap() = None;
    Ok(())
}
