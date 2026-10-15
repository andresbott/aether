//! The app's persisted settings: a small, non-secret JSON file in the app
//! config directory — the server the user picked and how the SPA signs in to
//! it. No credential ever lands here: the session cookie lives in the HTTP
//! plugin's cookie jar, a pasted API key in `credentials.json` (see
//! `credentials.rs`), and the connected client in memory (`session.rs`).
use serde::{Deserialize, Serialize};
use std::io::Write;
use std::path::Path;
use std::{fs, io};

const SETTINGS_FILE: &str = "settings.json";

/// How the SPA signs in to the saved server. `Cookie` is the native flow (a
/// login posted through the transport, the session cookie in the plugin's jar,
/// a spa token minted on /api/v0). `ApiKey` is a /rest-only session on a key
/// the user pasted, which `credentials.rs` stores; the SPA then never asks
/// /api/v0 for anything. The kind outlives the key — a logout or a revoked key
/// deletes the key, not the kind — so a portal server comes back to the key
/// card at the next launch instead of a wrong "Could not reach".
#[derive(Serialize, Deserialize, Clone, Copy, Debug, PartialEq, Eq, Default)]
#[serde(rename_all = "camelCase")]
pub enum SessionKind {
    #[default]
    Cookie,
    ApiKey,
}

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct Settings {
    /// The normalised address of the server the user picked, if any.
    pub server_url: Option<String>,
    /// How the SPA signs in to it.
    pub session_kind: SessionKind,
}

/// Whether saving `next` over `current` leaves the stored API key in place. A
/// key belongs to the server that issued it, and this is the one decision that
/// keeps a stored key off another server: it survives only the same server
/// saved again as a key session (Connect after an outage), never a different
/// server, a cookie session or no server at all.
pub fn keeps_key(current: &Settings, next: &Settings) -> bool {
    next.session_kind == SessionKind::ApiKey
        && next.server_url.is_some()
        && next.server_url == current.server_url
}

/// The settings saved in the config directory `dir`. A missing or unparsable
/// file yields the defaults, so a damaged file can never keep the app from
/// starting.
pub fn load_from(dir: &Path) -> Settings {
    let (settings, problem) = read_from(&dir.join(SETTINGS_FILE));
    if let Some(problem) = problem {
        eprintln!("settings: using defaults, {problem}");
    }
    settings
}

/// Saves `settings` in the config directory `dir`, creating it if needed.
pub fn save_to(dir: &Path, settings: &Settings) -> io::Result<()> {
    write_to(&dir.join(SETTINGS_FILE), settings)
}

/// Reads a settings file. A missing one is a first launch and yields the
/// defaults quietly; an unreadable or unparsable one yields them with the
/// problem to report.
fn read_from(path: &Path) -> (Settings, Option<String>) {
    let read = fs::read_to_string(path).and_then(|text| Ok(serde_json::from_str(&text)?));
    match read {
        Ok(settings) => (settings, None),
        Err(e) if e.kind() == io::ErrorKind::NotFound => (Settings::default(), None),
        Err(e) => (
            Settings::default(),
            Some(format!("cannot read {}: {e}", path.display())),
        ),
    }
}

/// Writes a settings file, creating its directory when it does not exist yet
/// (the config directory is absent on first launch). The text goes to a temp
/// file next to it, which then replaces the old file in one rename: a save cut
/// short leaves the old settings, never an empty or half-written file.
fn write_to(path: &Path, settings: &Settings) -> io::Result<()> {
    if let Some(dir) = path.parent() {
        fs::create_dir_all(dir)?;
    }
    let text = serde_json::to_string_pretty(settings)?;
    let temp = path.with_extension("json.tmp");
    let replaced = (|| -> io::Result<()> {
        let mut file = fs::File::create(&temp)?;
        file.write_all(text.as_bytes())?;
        file.sync_all()?;
        fs::rename(&temp, path)
    })();
    if replaced.is_err() {
        let _ = fs::remove_file(&temp);
    }
    replaced
}

#[cfg(test)]
mod tests {
    use super::*;

    use std::path::PathBuf;

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
        assert_eq!(none, r#"{"serverUrl":null,"sessionKind":"cookie"}"#);

        let some = Settings {
            server_url: Some("http://h:8075".into()),
            session_kind: SessionKind::ApiKey,
        };
        let some = serde_json::to_string(&some).unwrap();
        assert_eq!(
            some,
            r#"{"serverUrl":"http://h:8075","sessionKind":"apiKey"}"#
        );
    }

    #[test]
    fn settings_round_trip_through_the_file() {
        let dir = fresh_dir("round-trip");
        // neither the directory nor the file exist on first launch
        let path = dir.join("config").join(SETTINGS_FILE);

        let picked = Settings {
            server_url: Some("http://h:8075".into()),
            session_kind: SessionKind::ApiKey,
        };
        write_to(&path, &picked).unwrap();
        assert_eq!(read_from(&path), (picked, None));

        // clearing the server overwrites the file rather than leaving the old one
        write_to(&path, &Settings::default()).unwrap();
        assert_eq!(read_from(&path), (Settings::default(), None));

        fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn a_stored_key_stays_only_for_the_same_server_saved_again_as_a_key_session() {
        use SessionKind::{ApiKey, Cookie};
        fn saved(server: Option<&str>, session_kind: SessionKind) -> Settings {
            Settings {
                server_url: server.map(str::to_string),
                session_kind,
            }
        }
        let a = Some("http://a:8075");
        let b = Some("http://b:8075");

        // (saved now, saved next, whether the stored key stays)
        let cases = [
            // the same server, saved again as a key session (Connect after an outage)
            (saved(a, ApiKey), saved(a, ApiKey), true),
            // another server
            (saved(a, ApiKey), saved(b, ApiKey), false),
            // the same server as a cookie session (the password link)
            (saved(a, ApiKey), saved(a, Cookie), false),
            // no server at all
            (saved(None, ApiKey), saved(None, ApiKey), false),
            // a first server, which no key was stored for
            (saved(None, Cookie), saved(a, ApiKey), false),
        ];
        for (current, next, keeps) in cases {
            assert_eq!(keeps_key(&current, &next), keeps, "{current:?} -> {next:?}");
        }
    }

    #[test]
    fn a_missing_or_damaged_file_yields_the_defaults() {
        let dir = fresh_dir("fallback");
        let path = dir.join(SETTINGS_FILE);
        assert_eq!(read_from(&path).0, Settings::default(), "missing file");

        fs::create_dir_all(&dir).unwrap();
        fs::write(&path, "{ not json").unwrap();
        let (settings, problem) = read_from(&path);
        assert_eq!(settings, Settings::default(), "damaged file");
        // a damaged file is worth a line on stderr: the saved server is lost
        assert!(problem.is_some(), "a damaged file went unreported");

        fs::remove_dir_all(&dir).unwrap();
    }

    // Every first launch has no settings file: that is no news, and a "cannot
    // read" line on each of them would bury the times a file really is damaged.
    #[test]
    fn a_first_launch_reports_nothing() {
        let path = fresh_dir("first-launch").join(SETTINGS_FILE);

        assert_eq!(read_from(&path), (Settings::default(), None));
    }

    // A save that truncates the file and writes it again leaves it empty or half
    // written when the app dies in between, which reads as the defaults: the
    // saved server is forgotten. Replaced through a renamed temp file, it always
    // holds either the old settings or the new ones.
    #[cfg(unix)]
    #[test]
    fn saving_replaces_the_file_instead_of_rewriting_it_in_place() {
        use std::os::unix::fs::MetadataExt;
        let dir = fresh_dir("atomic");
        let path = dir.join(SETTINGS_FILE);
        write_to(&path, &Settings::default()).unwrap();
        let rewritten_in_place = fs::metadata(&path).unwrap().ino();
        let picked = Settings {
            server_url: Some("http://h:8075".into()),
            session_kind: SessionKind::Cookie,
        };

        write_to(&path, &picked).unwrap();

        assert_ne!(
            fs::metadata(&path).unwrap().ino(),
            rewritten_in_place,
            "the old file was rewritten in place"
        );
        assert_eq!(read_from(&path), (picked, None));
        // and no temp file is left behind next to it
        let names: Vec<_> = fs::read_dir(&dir)
            .unwrap()
            .map(|entry| entry.unwrap().file_name())
            .collect();
        assert_eq!(names, [SETTINGS_FILE]);
        fs::remove_dir_all(&dir).unwrap();
    }
}
