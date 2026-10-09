use serde::{Deserialize, Serialize};
use std::fs;
use tauri::{AppHandle, Manager};

const KEYRING_SERVICE: &str = "dev.andresbott.aether.player";
const CONN_FILE: &str = "connection.json";

#[derive(Serialize, Deserialize, Clone)]
pub struct StoredConnection {
    pub server_url: String,
    pub username: String,
}

fn conn_path(app: &AppHandle) -> Option<std::path::PathBuf> {
    app.path().app_config_dir().ok().map(|d| d.join(CONN_FILE))
}

pub fn save(app: &AppHandle, server_url: &str, username: &str, password: &str) {
    if let Some(path) = conn_path(app) {
        if let Some(dir) = path.parent() {
            let _ = fs::create_dir_all(dir);
        }
        let stored = StoredConnection {
            server_url: server_url.to_string(),
            username: username.to_string(),
        };
        if let Ok(json) = serde_json::to_string_pretty(&stored) {
            let _ = fs::write(&path, json);
        }
    }
    match keyring::Entry::new(KEYRING_SERVICE, username) {
        Ok(entry) => {
            if let Err(e) = entry.set_password(password) {
                eprintln!("keyring save failed: {e}");
            }
        }
        Err(e) => eprintln!("keyring unavailable: {e}"),
    }
}

pub fn load(app: &AppHandle) -> Option<(StoredConnection, String)> {
    let path = conn_path(app)?;
    let stored: StoredConnection = serde_json::from_str(&fs::read_to_string(path).ok()?).ok()?;
    let password = keyring::Entry::new(KEYRING_SERVICE, &stored.username)
        .ok()?
        .get_password()
        .ok()?;
    Some((stored, password))
}

/// Clear stored credentials (config file and keyring entry).
///
/// Removes the connection.json file in all cases. Attempts to remove the keyring entry
/// if the file is readable and parseable. If the connection.json file cannot be read or parsed,
/// the keyring entry cannot be located and will not be removed; it will be overwritten on the
/// next successful connection for the same username.
pub fn clear(app: &AppHandle) {
    if let Some(path) = conn_path(app) {
        let read_result = fs::read_to_string(&path).and_then(|s| {
            serde_json::from_str::<StoredConnection>(&s)
                .map_err(|e| std::io::Error::new(std::io::ErrorKind::InvalidData, e))
        });

        match read_result {
            Ok(stored) => {
                if let Ok(entry) = keyring::Entry::new(KEYRING_SERVICE, &stored.username) {
                    if let Err(e) = entry.delete_credential() {
                        eprintln!("credentials: failed to delete keyring entry: {e}");
                    }
                }
            }
            Err(e) => {
                eprintln!("credentials: cannot read stored connection; keyring entry (if any) not removed: {e}");
            }
        }
        let _ = fs::remove_file(path);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_keyring_service_name_is_correct() {
        assert_eq!(KEYRING_SERVICE, "dev.andresbott.aether.player");
    }
}
