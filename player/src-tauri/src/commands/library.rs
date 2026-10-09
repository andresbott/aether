use crate::error::AppError;
use crate::state::AppState;
use aether_core::subsonic::models::{Album, Artist, SearchResult};
use tauri::State;

#[tauri::command]
pub async fn get_artists(state: State<'_, AppState>) -> Result<Vec<Artist>, AppError> {
    Ok(state.client()?.get_artists().await?)
}

#[tauri::command]
pub async fn get_albums(
    state: State<'_, AppState>,
    list_type: String,
    size: u32,
    offset: u32,
) -> Result<Vec<Album>, AppError> {
    Ok(state.client()?.get_albums(&list_type, size, offset).await?)
}

#[tauri::command]
pub async fn get_album(state: State<'_, AppState>, id: String) -> Result<Album, AppError> {
    Ok(state.client()?.get_album(&id).await?)
}

#[tauri::command]
pub async fn search(state: State<'_, AppState>, query: String) -> Result<SearchResult, AppError> {
    Ok(state.client()?.search(&query).await?)
}
