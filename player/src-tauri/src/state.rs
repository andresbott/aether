use aether_core::error::CoreError;
use aether_core::player::{Player, StreamUrlResolver};
use aether_core::subsonic::SubsonicClient;
use std::sync::{Arc, RwLock};

pub struct AppState {
    pub connection: Arc<RwLock<Option<SubsonicClient>>>,
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
    pub fn new() -> Result<Self, CoreError> {
        let connection = Arc::new(RwLock::new(None));
        let engine = crate::engine::create_engine()?;
        let resolver = Arc::new(ConnectionResolver(connection.clone()));
        let player = Arc::new(Player::new(engine, resolver));
        Ok(Self { connection, player })
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
