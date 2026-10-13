use crate::settings::{SessionKind, Settings};
use aether_core::error::CoreError;
use aether_core::player::{Player, StreamUrlResolver};
use aether_core::subsonic::SubsonicClient;
use std::sync::{Arc, RwLock};

pub struct AppState {
    /// The authenticated client: built at startup from a stored API key, or
    /// handed over by the SPA (`set_session` in the cookie flow, `set_api_key`
    /// in a key session). Lock order: `settings` → `api_key` → `connection`.
    pub connection: Arc<RwLock<Option<SubsonicClient>>>,
    pub settings: RwLock<Settings>,
    /// The API key stored in `credentials.json`, mirrored in memory. Taken after
    /// `settings`, never before it.
    pub api_key: RwLock<Option<String>>,
    pub player: Arc<Player>,
}

/// Resolves stream URLs from the current connection (errors when disconnected).
pub struct ConnectionResolver(pub Arc<RwLock<Option<SubsonicClient>>>);

impl StreamUrlResolver for ConnectionResolver {
    fn stream_url(&self, track_id: &str) -> Result<String, CoreError> {
        self.0
            .read()
            .unwrap()
            .as_ref()
            .map(|c| c.stream_url(track_id))
            .ok_or(CoreError::NotConnected)
    }
}

impl AppState {
    /// Starts with the settings and key loaded at startup: connected when they
    /// name a server and a key for a key session, disconnected otherwise.
    pub fn new(settings: Settings, api_key: Option<String>) -> Result<Self, CoreError> {
        let connection = Arc::new(RwLock::new(connection_for(&settings, api_key.as_deref())));
        let engine = crate::engine::create_engine()?;
        let resolver = Arc::new(ConnectionResolver(connection.clone()));
        let player = Arc::new(Player::new(engine, resolver));
        Ok(Self {
            connection,
            settings: RwLock::new(settings),
            api_key: RwLock::new(api_key),
            player,
        })
    }

    /// Cloned client or NotConnected — commands call this.
    pub fn client(&self) -> Result<SubsonicClient, aether_core::error::CoreError> {
        self.connection
            .read()
            .unwrap()
            .clone()
            .ok_or(aether_core::error::CoreError::NotConnected)
    }
}

/// The client the stored state calls for: only a key session with both a server
/// and a key connects; the cookie flow's client arrives from the SPA. An address
/// that no longer parses (a hand-edited file) answers no client and says so on
/// stderr, so a bad file never keeps the app from starting. Pure, so it is
/// tested without an audio engine.
pub fn connection_for(settings: &Settings, api_key: Option<&str>) -> Option<SubsonicClient> {
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
    use crate::settings::SessionKind;

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
}
