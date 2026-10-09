//! The app's persisted settings: a small, non-secret JSON file in the app
//! config directory. Today that is only the server the user picked. The
//! session never lands here — it lives in memory (`AppState.connection`) and
//! in the HTTP plugin's cookie jar.
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};
use std::{fs, io};
use tauri::{AppHandle, Manager};

const SETTINGS_FILE: &str = "settings.json";

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct Settings {
    /// The normalised address of the server the user picked, if any.
    pub server_url: Option<String>,
}

fn settings_path(app: &AppHandle) -> io::Result<PathBuf> {
    app.path()
        .app_config_dir()
        .map(|dir| dir.join(SETTINGS_FILE))
        .map_err(|e| io::Error::other(e.to_string()))
}

/// The saved settings. A missing or unparsable file yields the defaults, so a
/// damaged file can never keep the app from starting.
pub fn load(app: &AppHandle) -> Settings {
    match settings_path(app) {
        Ok(path) => read_from(&path),
        Err(e) => {
            eprintln!("settings: no config directory, using defaults: {e}");
            Settings::default()
        }
    }
}

pub fn save(app: &AppHandle, settings: &Settings) -> io::Result<()> {
    write_to(&settings_path(app)?, settings)
}

/// Reads a settings file; a missing or unparsable one yields the defaults and
/// the reason goes to stderr.
fn read_from(path: &Path) -> Settings {
    let read = fs::read_to_string(path).and_then(|text| Ok(serde_json::from_str(&text)?));
    read.unwrap_or_else(|e| {
        eprintln!(
            "settings: using defaults, cannot read {}: {e}",
            path.display()
        );
        Settings::default()
    })
}

/// Writes a settings file, creating its directory when it does not exist yet
/// (the config directory is absent on first launch).
fn write_to(path: &Path, settings: &Settings) -> io::Result<()> {
    if let Some(dir) = path.parent() {
        fs::create_dir_all(dir)?;
    }
    fs::write(path, serde_json::to_string_pretty(settings)?)
}

#[cfg(test)]
mod tests {
    use super::*;

    /// A directory under the system temp dir that does not exist yet, unique
    /// to the calling test.
    fn fresh_dir(test: &str) -> PathBuf {
        let dir =
            std::env::temp_dir().join(format!("aether-settings-{}-{test}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        dir
    }

    #[test]
    fn settings_serialize_to_camel_case_json() {
        // the SPA reads this shape from get_settings / set_server_url
        let none = serde_json::to_string(&Settings::default()).unwrap();
        assert_eq!(none, r#"{"serverUrl":null}"#);

        let some = Settings {
            server_url: Some("http://h:8075".into()),
        };
        let some = serde_json::to_string(&some).unwrap();
        assert_eq!(some, r#"{"serverUrl":"http://h:8075"}"#);
    }

    #[test]
    fn settings_round_trip_through_the_file() {
        let dir = fresh_dir("round-trip");
        // neither the directory nor the file exist on first launch
        let path = dir.join("config").join(SETTINGS_FILE);

        let picked = Settings {
            server_url: Some("http://h:8075".into()),
        };
        write_to(&path, &picked).unwrap();
        assert_eq!(read_from(&path), picked);

        // clearing the server overwrites the file rather than leaving the old one
        write_to(&path, &Settings::default()).unwrap();
        assert_eq!(read_from(&path), Settings::default());

        fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn a_missing_or_damaged_file_yields_the_defaults() {
        let dir = fresh_dir("fallback");
        let path = dir.join(SETTINGS_FILE);
        assert_eq!(read_from(&path), Settings::default(), "missing file");

        fs::create_dir_all(&dir).unwrap();
        fs::write(&path, "{ not json").unwrap();
        assert_eq!(read_from(&path), Settings::default(), "damaged file");

        fs::remove_dir_all(&dir).unwrap();
    }
}
