//! The player's session: the saved settings, the stored API key and the client
//! they connect, behind the locks the IPC commands share. The commands are thin
//! wrappers around it, and it works on a plain config directory, so what it
//! guarantees is tested without a Tauri app or an audio engine.
use crate::credentials;
use crate::error::AppError;
use crate::settings::{self, SessionKind, Settings};
use aether_core::error::CoreError;
use aether_core::subsonic::{normalize_server_url, SubsonicClient};
use std::io;
use std::path::{Path, PathBuf};
use std::sync::{Arc, RwLock};

/// Lock order: `settings` → `api_key` → `connection`. Every method that takes
/// more than one takes them in that order, so none can deadlock another, and
/// the ones that change the server hold `settings` until the client is rebuilt,
/// so a session can never be stored for a server that is no longer the saved one.
pub struct Session {
    /// The app config directory, home of `settings.json` and `credentials.json`;
    /// None when the platform has none, and then every save fails.
    dir: Option<PathBuf>,
    settings: RwLock<Settings>,
    /// The key stored in `credentials.json`, mirrored in memory.
    api_key: RwLock<Option<String>>,
    /// The authenticated client: built from a stored API key (at startup, or by
    /// `set_api_key`) or from the spa token the SPA hands over (`set_session`).
    /// Shared with the player's stream URL resolver.
    connection: Arc<RwLock<Option<SubsonicClient>>>,
}

impl Session {
    /// The session the files in `dir` describe: connected at once when they name
    /// a key session with a server and a key, disconnected otherwise — the cookie
    /// flow's client arrives from the SPA.
    pub fn load(dir: Option<PathBuf>) -> Self {
        let (settings, api_key) = match &dir {
            Some(dir) => (settings::load_from(dir), credentials::load_from(dir)),
            None => (Settings::default(), None),
        };
        let connection = connection_for(&settings, api_key.as_deref());
        Self {
            dir,
            settings: RwLock::new(settings),
            api_key: RwLock::new(api_key),
            connection: Arc::new(RwLock::new(connection)),
        }
    }

    /// The client slot, for the player's stream URL resolver.
    pub fn connection(&self) -> Arc<RwLock<Option<SubsonicClient>>> {
        self.connection.clone()
    }

    /// A clone of the connected client, or NotConnected.
    pub fn client(&self) -> Result<SubsonicClient, CoreError> {
        self.connection
            .read()
            .unwrap()
            .clone()
            .ok_or(CoreError::NotConnected)
    }

    pub fn settings(&self) -> Settings {
        self.settings.read().unwrap().clone()
    }

    pub fn api_key(&self) -> Option<String> {
        self.api_key.read().unwrap().clone()
    }

    /// Remembers the server the user picked, in its normalised form (so
    /// `http://h:8075/` is saved and returned as `http://h:8075`), and how the
    /// SPA signs in to it. A stored key belongs to the server that issued it, so
    /// it is deleted — unless the same server is saved again for a key session,
    /// which is what Connect does after an outage: that never costs the key.
    pub fn set_server_url(
        &self,
        url: &str,
        session_kind: SessionKind,
    ) -> Result<Settings, AppError> {
        let server_url = normalize_server_url(url)?;
        let mut current = self.settings.write().unwrap();
        let next = Settings {
            server_url: Some(server_url),
            session_kind,
        };
        self.replace_settings(&mut current, next)
    }

    /// Switches how the SPA signs in to the saved server (the login view's "Use
    /// an API key instead" / "Sign in with a password instead"). Leaving the key
    /// session deletes the stored key: a secret nothing uses must not stay on disk.
    pub fn set_session_kind(&self, session_kind: SessionKind) -> Result<Settings, AppError> {
        let mut current = self.settings.write().unwrap();
        let next = Settings {
            server_url: current.server_url.clone(),
            session_kind,
        };
        self.replace_settings(&mut current, next)
    }

    /// Forgets the server, the sign-in kind and any stored key (Change server).
    pub fn clear_server_url(&self) -> Result<Settings, AppError> {
        let mut current = self.settings.write().unwrap();
        self.replace_settings(&mut current, Settings::default())
    }

    /// Persists `next` and makes it the current settings. The stored key is
    /// deleted unless `settings::keeps_key` keeps it, and the connection is
    /// rebuilt from what is left: a key session that kept its key stays
    /// connected, everything else disconnects (a session belongs to the server
    /// and the sign-in it was made for). The key is deleted before the settings
    /// are written, so a failure in between errs toward less access. When a
    /// write fails, memory stays as it was, but the key file may already be gone.
    ///
    /// `current` is the settings write lock the caller holds; the key lock is
    /// taken here, after it, as the lock order says.
    fn replace_settings(
        &self,
        current: &mut Settings,
        next: Settings,
    ) -> Result<Settings, AppError> {
        let keep_key = settings::keeps_key(current, &next);
        let dir = self.dir()?;
        let mut stored_key = self.api_key.write().unwrap();
        if !keep_key {
            credentials::clear_in(dir).map_err(internal)?;
        }
        settings::save_to(dir, &next).map_err(internal)?;
        if !keep_key {
            *stored_key = None;
        }
        *current = next.clone();
        *self.connection.write().unwrap() = connection_for(&next, stored_key.as_deref());
        Ok(next)
    }

    /// Stores the key the SPA verified against the saved server, makes this a
    /// key session and connects the client with it — no round trip, the SPA did
    /// the verifying. `NotConnected` without a server, `InvalidServerUrl` for a
    /// hand-edited address that no longer parses. When a file cannot be written,
    /// memory stays as it was. The settings go first and the key second, so when
    /// the key write fails `settings.json` may already say `apiKey` with no key
    /// stored: a key session without a key, which only brings the key card back.
    pub fn set_api_key(&self, api_key: String) -> Result<Settings, AppError> {
        let mut current = self.settings.write().unwrap();
        let mut stored_key = self.api_key.write().unwrap();
        let server_url = current
            .server_url
            .as_deref()
            .ok_or(CoreError::NotConnected)?;
        let client = SubsonicClient::with_api_key(server_url, &api_key)?;
        let next = Settings {
            server_url: current.server_url.clone(),
            session_kind: SessionKind::ApiKey,
        };
        let dir = self.dir()?;
        if *current != next {
            settings::save_to(dir, &next).map_err(internal)?;
        }
        credentials::save_to(dir, &api_key).map_err(internal)?;
        *current = next.clone();
        *stored_key = Some(api_key);
        *self.connection.write().unwrap() = Some(client);
        Ok(next)
    }

    /// Forgets the stored key — a logout, or a key the server stopped accepting
    /// — and drops the connection. The session kind stays, so the key card comes
    /// back rather than the server step.
    pub fn clear_api_key(&self) -> Result<(), AppError> {
        // Never read: taken first only to keep the lock order.
        let _settings = self.settings.read().unwrap();
        let mut stored_key = self.api_key.write().unwrap();
        credentials::clear_in(self.dir()?).map_err(internal)?;
        *stored_key = None;
        *self.connection.write().unwrap() = None;
        Ok(())
    }

    /// Connects the client to the saved server with the spa token the SPA
    /// minted after signing in (the cookie flow's hand-off). The token is not
    /// checked — the SPA just minted it — so there is no round trip.
    /// `NotConnected` when no server is saved.
    pub fn set_session(&self, api_key: &str) -> Result<(), AppError> {
        // The settings stay read-locked until the client is stored, so a server
        // change cannot slip in between and leave a session for the old server.
        let settings = self.settings.read().unwrap();
        let server_url = settings
            .server_url
            .as_deref()
            .ok_or(CoreError::NotConnected)?;
        let client = SubsonicClient::with_api_key(server_url, api_key)?;
        *self.connection.write().unwrap() = Some(client);
        Ok(())
    }

    pub fn clear_session(&self) {
        *self.connection.write().unwrap() = None;
    }

    fn dir(&self) -> Result<&Path, AppError> {
        self.dir
            .as_deref()
            .ok_or_else(|| AppError::internal("no config directory"))
    }
}

fn internal(e: io::Error) -> AppError {
    AppError::internal(e.to_string())
}

/// The client the stored state calls for: only a key session with both a server
/// and a key connects; the cookie flow's client arrives from the SPA. An address
/// that no longer parses (a hand-edited file) answers no client and says so on
/// stderr, so a bad file never keeps the app from starting.
fn connection_for(settings: &Settings, api_key: Option<&str>) -> Option<SubsonicClient> {
    if settings.session_kind != SessionKind::ApiKey {
        return None;
    }
    let server_url = settings.server_url.as_deref()?;
    let api_key = api_key?;
    match SubsonicClient::with_api_key(server_url, api_key) {
        Ok(client) => Some(client),
        Err(e) => {
            eprintln!("settings: saved server address refused, starting disconnected: {e}");
            None
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::error::ErrorCode;
    use std::fs;

    const A: &str = "http://a:8075";
    const B: &str = "http://b:8075";

    /// A config directory under the system temp dir that does not exist yet,
    /// unique to the calling test — a first launch has none either.
    fn fresh_dir(test: &str) -> PathBuf {
        let dir =
            std::env::temp_dir().join(format!("aether-session-{}-{test}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        dir
    }

    // The SPA hands over its spa token for whatever server Rust has saved; with
    // none saved there is nobody to build a client for.
    #[test]
    fn set_session_answers_not_connected_without_a_server() {
        let session = Session::load(Some(fresh_dir("set-session-no-server")));

        let err = session.set_session("aether_t0k3n").unwrap_err();

        assert!(matches!(err.code, ErrorCode::NotConnected), "{err:?}");
        assert!(
            session.client().is_err(),
            "a client was stored without a server"
        );
    }

    #[test]
    fn set_api_key_answers_not_connected_without_a_server() {
        let dir = fresh_dir("set-api-key-no-server");
        let session = Session::load(Some(dir.clone()));

        let err = session.set_api_key("aether_k3y".into()).unwrap_err();

        assert!(matches!(err.code, ErrorCode::NotConnected), "{err:?}");
        assert_eq!(session.api_key(), None);
        assert!(
            !dir.join("credentials.json").exists(),
            "a key was stored without a server"
        );
    }

    // A session belongs to the server it was made for: a client for A must not
    // stay connected once B is the saved server.
    #[test]
    fn saving_another_server_drops_the_session() {
        let dir = fresh_dir("switch-drops-session");
        let session = Session::load(Some(dir.clone()));
        session.set_server_url(A, SessionKind::Cookie).unwrap();
        session.set_session("aether_t0k3n").unwrap();

        session.set_server_url(B, SessionKind::Cookie).unwrap();

        assert!(
            session.client().is_err(),
            "A's session survived the switch to B"
        );
        fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn forgetting_the_server_drops_the_session() {
        let dir = fresh_dir("clear-drops-session");
        let session = Session::load(Some(dir.clone()));
        session.set_server_url(A, SessionKind::Cookie).unwrap();
        session.set_session("aether_t0k3n").unwrap();

        let saved = session.clear_server_url().unwrap();

        assert_eq!(saved, Settings::default());
        assert!(
            session.client().is_err(),
            "the session survived Change server"
        );
        fs::remove_dir_all(&dir).unwrap();
    }

    // Connect after an outage saves the same server again as a key session: that
    // must never cost the key the user pasted for it.
    #[test]
    fn the_same_server_saved_again_as_a_key_session_keeps_its_key() {
        let dir = fresh_dir("same-server-keeps-key");
        let session = Session::load(Some(dir.clone()));
        session.set_server_url(A, SessionKind::ApiKey).unwrap();
        session.set_api_key("aether_k3y".into()).unwrap();

        session.set_server_url(A, SessionKind::ApiKey).unwrap();

        assert_eq!(session.api_key().as_deref(), Some("aether_k3y"));
        assert!(
            dir.join("credentials.json").exists(),
            "the key file is gone"
        );
        assert_eq!(session.client().unwrap().base_url_str(), A);
        fs::remove_dir_all(&dir).unwrap();
    }

    // A key belongs to the server that issued it: saving another one, even as a
    // key session, must never leave the old key where the new server's client
    // would send it.
    #[test]
    fn another_server_costs_the_stored_key() {
        let dir = fresh_dir("other-server-costs-key");
        let session = Session::load(Some(dir.clone()));
        session.set_server_url(A, SessionKind::ApiKey).unwrap();
        session.set_api_key("aether_k3y".into()).unwrap();

        session.set_server_url(B, SessionKind::ApiKey).unwrap();

        assert_eq!(session.api_key(), None);
        assert!(
            !dir.join("credentials.json").exists(),
            "A's key is still on disk"
        );
        assert!(session.client().is_err(), "a client still carries A's key");
        fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn a_failed_settings_write_leaves_settings_and_session_untouched() {
        let dir = fresh_dir("failed-write");
        let session = Session::load(Some(dir.clone()));
        session.set_server_url(A, SessionKind::Cookie).unwrap();
        session.set_session("aether_t0k3n").unwrap();
        // The config directory turns into a plain file, so nothing can be written
        // into it — and it fails the same way for root, unlike a read-only mode.
        fs::remove_dir_all(&dir).unwrap();
        fs::write(&dir, "").unwrap();

        let err = session.set_server_url(B, SessionKind::Cookie).unwrap_err();

        assert!(matches!(err.code, ErrorCode::Internal), "{err:?}");
        assert_eq!(session.settings().server_url.as_deref(), Some(A));
        assert_eq!(session.client().unwrap().base_url_str(), A);
        fs::remove_file(&dir).unwrap();
    }

    fn key_session(server: Option<&str>) -> Settings {
        Settings {
            server_url: server.map(str::to_string),
            session_kind: SessionKind::ApiKey,
        }
    }

    #[test]
    fn connects_from_a_stored_key_in_a_key_session() {
        let client = connection_for(&key_session(Some("http://h:8075")), Some("aether_k3y"))
            .expect("a key session with a server and a key connects");

        assert_eq!(client.base_url_str(), "http://h:8075");
        // the key is installed on the client, not only the address
        assert!(
            client.stream_url("t1").contains("apiKey=aether_k3y"),
            "the client does not send the stored key"
        );
    }

    #[test]
    fn starts_disconnected_without_a_key_a_server_or_a_key_session() {
        assert!(connection_for(&key_session(Some("http://h:8075")), None).is_none());
        assert!(connection_for(&key_session(None), Some("aether_k3y")).is_none());
        // the cookie flow's client arrives from the SPA (set_session), not from a file
        let cookie = Settings {
            server_url: Some("http://h:8075".into()),
            session_kind: SessionKind::Cookie,
        };
        assert!(connection_for(&cookie, Some("aether_k3y")).is_none());
    }

    // A hand-edited settings file: the app starts, disconnected, instead of
    // failing at setup.
    #[test]
    fn starts_disconnected_on_an_address_that_no_longer_parses() {
        assert!(connection_for(&key_session(Some("not a url")), Some("aether_k3y")).is_none());
    }

    // What setup does at every launch: a key session saved last time connects
    // straight away from the files, with no SPA involved.
    #[test]
    fn a_key_session_connects_again_at_the_next_launch() {
        let dir = fresh_dir("next-launch");
        let first = Session::load(Some(dir.clone()));
        first.set_server_url(A, SessionKind::ApiKey).unwrap();
        first.set_api_key("aether_k3y".into()).unwrap();
        drop(first);

        let next = Session::load(Some(dir.clone()));

        assert_eq!(
            next.settings(),
            Settings {
                server_url: Some(A.into()),
                session_kind: SessionKind::ApiKey,
            }
        );
        assert_eq!(next.api_key().as_deref(), Some("aether_k3y"));
        assert_eq!(next.client().unwrap().base_url_str(), A);
        fs::remove_dir_all(&dir).unwrap();
    }
}
