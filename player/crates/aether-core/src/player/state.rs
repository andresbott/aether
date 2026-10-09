use serde::Serialize;

#[derive(Debug, Clone, Serialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct PlayerState {
    pub track_id: Option<String>,
    pub position_ms: u64,
    pub paused: bool,
    pub stopped: bool,
    pub volume: f32,
    pub queue: Vec<String>,
    pub queue_index: Option<usize>,
}
