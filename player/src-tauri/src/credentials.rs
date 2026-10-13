//! The API key the user pasted into the login view's key card, kept on its own
//! in the app config directory — never in `settings.json`, which stays
//! non-secret and safe to paste into a bug report. Written readable by its
//! owner only on Unix. Whether the key is in use is the settings' `SessionKind`;
//! this file exists only while a key is stored.
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};
use std::{fs, io};
use tauri::{AppHandle, Manager};

const CREDENTIALS_FILE: &str = "credentials.json";

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Credentials {
    api_key: String,
}

fn credentials_path(app: &AppHandle) -> io::Result<PathBuf> {
    app.path()
        .app_config_dir()
        .map(|dir| dir.join(CREDENTIALS_FILE))
        .map_err(|e| io::Error::other(e.to_string()))
}

/// The stored key, if any. A missing, damaged or empty file reads as no key:
/// the login view then asks for one, and the app starts either way.
pub fn load(app: &AppHandle) -> Option<String> {
    match credentials_path(app) {
        Ok(path) => read_from(&path),
        Err(e) => {
            eprintln!("credentials: no config directory, no key: {e}");
            None
        }
    }
}

pub fn save(app: &AppHandle, api_key: &str) -> io::Result<()> {
    write_to(&credentials_path(app)?, api_key)
}

/// Deletes the stored key. A file that is already gone is not an error.
pub fn clear(app: &AppHandle) -> io::Result<()> {
    remove(&credentials_path(app)?)
}

fn read_from(path: &Path) -> Option<String> {
    let text = match fs::read_to_string(path) {
        Ok(text) => text,
        Err(e) if e.kind() == io::ErrorKind::NotFound => return None,
        Err(e) => {
            eprintln!("credentials: cannot read {}: {e}", path.display());
            return None;
        }
    };
    match serde_json::from_str::<Credentials>(&text) {
        Ok(c) if !c.api_key.is_empty() => Some(c.api_key),
        Ok(_) => None,
        Err(e) => {
            eprintln!(
                "credentials: ignoring damaged {}: {}",
                path.display(),
                parse_failure(&e)
            );
            None
        }
    }
}

/// What kind of parse error and where, without serde's own message: that quotes
/// the value when the file holds a bare JSON string, which would put the key in
/// the log.
fn parse_failure(e: &serde_json::Error) -> String {
    format!(
        "{:?} error at line {}, column {}",
        e.classify(),
        e.line(),
        e.column()
    )
}

/// Writes the key, creating the config directory when it does not exist yet.
fn write_to(path: &Path, api_key: &str) -> io::Result<()> {
    if let Some(dir) = path.parent() {
        fs::create_dir_all(dir)?;
    }
    let text = serde_json::to_string_pretty(&Credentials {
        api_key: api_key.to_string(),
    })?;
    write_private(path, &text)
}

/// Writes the file readable by its owner only. `.mode` applies only when the
/// file is created, so a file that already exists keeps the permissions it
/// had: the mode is set again on the open handle, before the key is written, and
/// the key never sits in a file wider than that, not even for an instant.
#[cfg(unix)]
fn write_private(path: &Path, text: &str) -> io::Result<()> {
    use std::io::Write;
    use std::os::unix::fs::{OpenOptionsExt, PermissionsExt};
    let mut file = fs::OpenOptions::new()
        .write(true)
        .create(true)
        .truncate(true)
        .mode(0o600)
        .open(path)?;
    file.set_permissions(fs::Permissions::from_mode(0o600))?;
    file.write_all(text.as_bytes())
}

/// Windows: the per-user AppData directory is the protection.
#[cfg(not(unix))]
fn write_private(path: &Path, text: &str) -> io::Result<()> {
    fs::write(path, text)
}

fn remove(path: &Path) -> io::Result<()> {
    match fs::remove_file(path) {
        Ok(()) => Ok(()),
        Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(()),
        Err(e) => Err(e),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// A directory under the system temp dir that does not exist yet, unique
    /// to the calling test.
    fn fresh_dir(test: &str) -> PathBuf {
        let dir =
            std::env::temp_dir().join(format!("aether-credentials-{}-{test}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        dir
    }

    #[test]
    fn a_key_round_trips_through_the_file() {
        let dir = fresh_dir("round-trip");
        // the config directory may not exist yet (a first launch)
        let path = dir.join("config").join(CREDENTIALS_FILE);

        write_to(&path, "aether_k3y").unwrap();

        assert_eq!(read_from(&path), Some("aether_k3y".into()));
        // JSON with one camelCase field and nothing else: the shape is a contract
        let text = fs::read_to_string(&path).unwrap();
        let v: serde_json::Value = serde_json::from_str(&text).unwrap();
        assert_eq!(v, serde_json::json!({ "apiKey": "aether_k3y" }));
        fs::remove_dir_all(&dir).unwrap();
    }

    #[cfg(unix)]
    #[test]
    fn the_file_is_readable_by_its_owner_only() {
        use std::os::unix::fs::PermissionsExt;
        let dir = fresh_dir("mode");
        let path = dir.join(CREDENTIALS_FILE);

        write_to(&path, "first").unwrap();
        assert_eq!(
            fs::metadata(&path).unwrap().permissions().mode() & 0o777,
            0o600
        );

        // a file left wider by something else is tightened on the next write
        fs::set_permissions(&path, fs::Permissions::from_mode(0o644)).unwrap();
        write_to(&path, "second").unwrap();
        assert_eq!(
            fs::metadata(&path).unwrap().permissions().mode() & 0o777,
            0o600
        );
        assert_eq!(read_from(&path), Some("second".into()));
        fs::remove_dir_all(&dir).unwrap();
    }

    // A damaged file must never keep the app from starting; reading "no key"
    // sends the user to the key card instead.
    #[test]
    fn a_missing_damaged_or_empty_file_reads_as_no_key() {
        let dir = fresh_dir("fallback");
        let path = dir.join(CREDENTIALS_FILE);
        assert_eq!(read_from(&path), None, "missing file");

        fs::create_dir_all(&dir).unwrap();
        fs::write(&path, "{ not json").unwrap();
        assert_eq!(read_from(&path), None, "damaged file");

        fs::write(&path, r#"{"apiKey":""}"#).unwrap();
        assert_eq!(read_from(&path), None, "empty key");
        fs::remove_dir_all(&dir).unwrap();
    }

    // The log line for a damaged file must not quote the file: serde's own
    // message repeats a bare JSON string, which here would be the key itself.
    #[test]
    fn the_note_on_a_damaged_file_never_quotes_it() {
        // no `unwrap_err()`: that needs `Credentials: Debug`, kept off so the
        // key cannot reach a `{:?}`
        let err = serde_json::from_str::<Credentials>(r#""aether_k3y""#)
            .err()
            .expect("a bare string is not a credentials file");
        let note = parse_failure(&err);
        assert!(
            !note.contains("aether_k3y"),
            "the api key leaked into the log line: {note}"
        );
    }

    #[test]
    fn clearing_removes_the_file_and_tolerates_its_absence() {
        let dir = fresh_dir("clear");
        let path = dir.join(CREDENTIALS_FILE);
        write_to(&path, "aether_k3y").unwrap();

        remove(&path).unwrap();
        assert!(!path.exists());

        // logout after logout: the second delete finds nothing, and that is fine
        remove(&path).unwrap();
        fs::remove_dir_all(&dir).unwrap();
    }
}
