use crate::error::AppError;
use crate::state::AppState;
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

/// Connects the Rust-side client to the saved server with the spa token the SPA
/// minted after signing in. The token is not checked — no round trip — and
/// `NotConnected` answers when no server has been picked yet.
#[tauri::command]
pub fn set_session(state: State<'_, AppState>, api_key: String) -> Result<(), AppError> {
    state.session.set_session(&api_key)
}

#[tauri::command]
pub fn clear_session(state: State<'_, AppState>) {
    state.session.clear_session();
}

#[tauri::command]
pub fn connection_status(state: State<'_, AppState>) -> ConnectionStatus {
    match state.session.client() {
        Ok(c) => ConnectionStatus::Connected {
            server_url: c.base_url_str(),
        },
        Err(_) => ConnectionStatus::Disconnected,
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
