use crate::session::Session;
use aether_core::error::CoreError;
use aether_core::player::{Player, StreamUrlResolver};
use aether_core::subsonic::SubsonicClient;
use std::sync::{Arc, RwLock};

pub struct AppState {
    /// The saved server, the stored key and the connected client (see
    /// `session.rs` for the lock order).
    pub session: Session,
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
    /// The app's state around the session loaded at startup; the player
    /// resolves its stream URLs through that session's client.
    pub fn new(session: Session) -> Result<Self, CoreError> {
        let engine = crate::engine::create_engine()?;
        let resolver = Arc::new(ConnectionResolver(session.connection()));
        let player = Arc::new(Player::new(engine, resolver));
        Ok(Self { session, player })
    }

    /// Cloned client or NotConnected — commands call this.
    pub fn client(&self) -> Result<SubsonicClient, CoreError> {
        self.session.client()
    }
}
