use crate::error::CoreError;
use std::time::Duration;

/// Manages playback of audio streams.
///
/// # Concurrency Contract
///
/// Implementations MUST NOT call back into `Player`. The `Player` holds its internal lock
/// during all engine calls; a call-back would result in a deadlock.
pub trait PlaybackEngine: Send + Sync {
    /// Start playing the given stream URL from the beginning.
    ///
    /// This method may block (e.g., while buffering or downloading). The `Player` serializes
    /// all engine calls via its internal lock, so other `Player` methods like `state()` and
    /// `poll()` will stall until `load` returns.
    fn load(&self, url: &str) -> Result<(), CoreError>;
    fn pause(&self);
    fn resume(&self);
    fn stop(&self);
    fn seek(&self, pos: Duration) -> Result<(), CoreError>;
    /// volume in 0.0..=1.0
    fn set_volume(&self, volume: f32);
    /// Get the current playback position.
    ///
    /// This method MUST be fast and non-blocking. It is called frequently (state polling)
    /// while the `Player` holds its internal lock.
    fn position(&self) -> Duration;
    /// Check if the engine is idle.
    ///
    /// This method MUST be fast and non-blocking. It is called frequently (state polling)
    /// while the `Player` holds its internal lock. Returns true when nothing is loaded or
    /// the loaded track finished.
    fn is_idle(&self) -> bool;
}

pub trait StreamUrlResolver: Send + Sync {
    fn stream_url(&self, track_id: &str) -> Result<String, CoreError>;
}
