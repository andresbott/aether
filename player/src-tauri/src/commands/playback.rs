use crate::error::AppError;
use crate::state::AppState;
use aether_core::player::PlayerState;
use std::time::Duration;
use tauri::State;

#[tauri::command]
pub async fn play_track(state: State<'_, AppState>, id: String) -> Result<(), AppError> {
    let player = state.player.clone();
    tauri::async_runtime::spawn_blocking(move || player.play_track(&id))
        .await
        .map_err(|e| AppError::from(aether_core::error::CoreError::Engine(e.to_string())))??;
    Ok(())
}

#[tauri::command]
pub async fn queue_set(state: State<'_, AppState>, ids: Vec<String>) -> Result<(), AppError> {
    let player = state.player.clone();
    tauri::async_runtime::spawn_blocking(move || player.queue_set(ids))
        .await
        .map_err(|e| AppError::from(aether_core::error::CoreError::Engine(e.to_string())))??;
    Ok(())
}

#[tauri::command]
pub async fn queue_next(state: State<'_, AppState>) -> Result<(), AppError> {
    let player = state.player.clone();
    tauri::async_runtime::spawn_blocking(move || player.queue_next())
        .await
        .map_err(|e| AppError::from(aether_core::error::CoreError::Engine(e.to_string())))??;
    Ok(())
}

#[tauri::command]
pub async fn queue_prev(state: State<'_, AppState>) -> Result<(), AppError> {
    let player = state.player.clone();
    tauri::async_runtime::spawn_blocking(move || player.queue_prev())
        .await
        .map_err(|e| AppError::from(aether_core::error::CoreError::Engine(e.to_string())))??;
    Ok(())
}

#[tauri::command]
pub fn queue_get(state: State<'_, AppState>) -> PlayerState {
    state.player.state()
}

#[tauri::command]
pub fn pause(state: State<'_, AppState>) {
    state.player.pause();
}

#[tauri::command]
pub fn resume(state: State<'_, AppState>) {
    state.player.resume();
}

#[tauri::command]
pub fn stop(state: State<'_, AppState>) {
    state.player.stop();
}

#[tauri::command]
pub fn seek(state: State<'_, AppState>, position_ms: u64) -> Result<(), AppError> {
    state
        .player
        .seek(Duration::from_millis(position_ms))
        .map_err(AppError::from)
}

#[tauri::command]
pub fn set_volume(state: State<'_, AppState>, volume: f32) {
    state.player.set_volume(volume);
}
