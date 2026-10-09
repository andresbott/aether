use crate::error::CoreError;
use std::path::PathBuf;

/// Offline track cache boundary. Bootstrap ships only the no-op impl;
/// a disk-backed impl is a follow-up plan.
pub trait TrackCache: Send + Sync {
    /// Path to a cached copy of the track, if present.
    fn get(&self, track_id: &str) -> Option<PathBuf>;
    /// Store raw track bytes; returns the cached file path.
    fn put(&self, track_id: &str, bytes: &[u8]) -> Result<PathBuf, CoreError>;
}

pub struct NoopCache;

impl TrackCache for NoopCache {
    fn get(&self, _track_id: &str) -> Option<PathBuf> {
        None
    }
    fn put(&self, _track_id: &str, _bytes: &[u8]) -> Result<PathBuf, CoreError> {
        Err(CoreError::Engine("cache not implemented".into()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn noop_cache_never_hits() {
        let c = NoopCache;
        assert!(c.get("t1").is_none());
        assert!(c.put("t1", b"xx").is_err());
    }
}
