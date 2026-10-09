//! Stub. Follow-up plan: Tauri mobile plugin bridging to ExoPlayer/Media3.
use aether_core::error::CoreError;
use aether_core::player::PlaybackEngine;
use std::time::Duration;

pub struct AndroidEngine;

impl AndroidEngine {
    pub fn new() -> Self {
        Self
    }
}

impl Default for AndroidEngine {
    fn default() -> Self {
        Self::new()
    }
}

impl PlaybackEngine for AndroidEngine {
    fn load(&self, _url: &str) -> Result<(), CoreError> {
        Err(CoreError::Engine("android engine not implemented".into()))
    }
    fn pause(&self) {}
    fn resume(&self) {}
    fn stop(&self) {}
    fn seek(&self, _pos: Duration) -> Result<(), CoreError> {
        Err(CoreError::Engine("android engine not implemented".into()))
    }
    fn set_volume(&self, _volume: f32) {}
    fn position(&self) -> Duration {
        Duration::ZERO
    }
    fn is_idle(&self) -> bool {
        true
    }
}
