use crate::error::AppError;
use crate::state::AppState;
use aether_core::error::CoreError;
use aether_core::subsonic::SubsonicClient;
use serde::Serialize;
use tauri::State;

#[derive(Serialize)]
#[serde(rename_all = "camelCase", tag = "status")]
pub enum ConnectionStatus {
    #[serde(rename = "disconnected")]
    Disconnected,
    #[serde(rename = "connected")]
    Connected {
        #[serde(rename = "serverUrl")]
        server_url: String,
    },
}

/// Authenticates the Rust-side client against the configured server with the
/// API key the SPA minted after signing in. Fails with `NotConnected` when no
/// server has been picked yet.
#[tauri::command]
pub fn set_session(state: State<'_, AppState>, api_key: String) -> Result<(), AppError> {
    // The settings stay read-locked until the client is stored, so a server
    // change cannot slip in between and leave a session for the old server.
    let settings = state.settings.read().unwrap();
    let server_url = settings
        .server_url
        .as_deref()
        .ok_or(CoreError::NotConnected)?;
    let client = SubsonicClient::with_api_key(server_url, &api_key)?;
    *state.connection.write().unwrap() = Some(client);
    Ok(())
}

#[tauri::command]
pub fn clear_session(state: State<'_, AppState>) {
    *state.connection.write().unwrap() = None;
}

#[tauri::command]
pub fn connection_status(state: State<'_, AppState>) -> ConnectionStatus {
    match state.connection.read().unwrap().as_ref() {
        Some(c) => ConnectionStatus::Connected {
            server_url: c.base_url_str(),
        },
        None => ConnectionStatus::Disconnected,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_disconnected_serializes_to_lowercase_tag() {
        let status = ConnectionStatus::Disconnected;
        let json = serde_json::to_string(&status).expect("serialization failed");
        assert_eq!(json, r#"{"status":"disconnected"}"#);
    }

    #[test]
    fn test_connected_serializes_with_lowercase_tag_and_camel_case_fields() {
        let status = ConnectionStatus::Connected {
            server_url: "http://subsonic.example.com".to_string(),
        };
        let json = serde_json::to_string(&status).expect("serialization failed");
        let parsed: serde_json::Value = serde_json::from_str(&json).expect("json parsing failed");

        // lowercase status tag, camelCase serverUrl, and nothing else (no
        // username: the session is an API key, not a login)
        assert_eq!(
            parsed,
            serde_json::json!({
                "status": "connected",
                "serverUrl": "http://subsonic.example.com",
            })
        );
    }
}
