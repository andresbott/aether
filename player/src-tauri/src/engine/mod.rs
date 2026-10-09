use aether_core::error::CoreError;
use aether_core::player::PlaybackEngine;
use std::sync::Arc;

#[cfg(not(any(target_os = "android", target_os = "ios")))]
mod desktop;
#[cfg(not(any(target_os = "android", target_os = "ios")))]
pub fn create_engine() -> Result<Arc<dyn PlaybackEngine>, CoreError> {
    Ok(Arc::new(desktop::RodioEngine::start()?))
}

#[cfg(target_os = "android")]
mod android;
#[cfg(target_os = "android")]
pub fn create_engine() -> Result<Arc<dyn PlaybackEngine>, CoreError> {
    Ok(Arc::new(android::AndroidEngine::new()))
}

#[cfg(target_os = "ios")]
mod ios;
#[cfg(target_os = "ios")]
pub fn create_engine() -> Result<Arc<dyn PlaybackEngine>, CoreError> {
    Ok(Arc::new(ios::IosEngine::new()))
}
