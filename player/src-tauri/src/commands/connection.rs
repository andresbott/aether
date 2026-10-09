use crate::error::AppError;
use crate::state::AppState;
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
        username: String,
    },
}

#[tauri::command]
pub async fn connect(
    app: tauri::AppHandle,
    state: State<'_, AppState>,
    server_url: String,
    username: String,
    password: String,
) -> Result<(), AppError> {
    let client = SubsonicClient::new(&server_url, &username, &password)?;
    client.ping().await?; // validates credentials
    *state.connection.write().unwrap() = Some(client);
    crate::credentials::save(&app, &server_url, &username, &password);
    Ok(())
}

#[tauri::command]
pub fn connection_status(state: State<'_, AppState>) -> ConnectionStatus {
    match state.connection.read().unwrap().as_ref() {
        Some(c) => ConnectionStatus::Connected {
            server_url: c.base_url_str(),
            username: c.username_str(),
        },
        None => ConnectionStatus::Disconnected,
    }
}

#[tauri::command]
pub fn disconnect(app: tauri::AppHandle, state: State<'_, AppState>) {
    *state.connection.write().unwrap() = None;
    crate::credentials::clear(&app);
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
            username: "testuser".to_string(),
        };
        let json = serde_json::to_string(&status).expect("serialization failed");
        let parsed: serde_json::Value = serde_json::from_str(&json).expect("json parsing failed");

        // Verify status tag is lowercase
        assert_eq!(parsed["status"], "connected");
        // Verify serverUrl is camelCase
        assert_eq!(parsed["serverUrl"], "http://subsonic.example.com");
        // Verify username field exists
        assert_eq!(parsed["username"], "testuser");
    }
}
