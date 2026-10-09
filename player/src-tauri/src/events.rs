use crate::state::AppState;
use tauri::{AppHandle, Emitter, Manager};

pub const PLAYER_STATE_EVENT: &str = "player://state";

/// Spawns the poll loop: every 500ms advance the queue if the track ended
/// and emit the full player state to the UI.
pub fn spawn_state_emitter(app: AppHandle) {
    std::thread::Builder::new()
        .name("aether-state-emitter".into())
        .spawn(move || {
            let mut last = None;
            loop {
                {
                    let state = app.state::<AppState>();
                    state.player.poll();
                    let s = state.player.state();
                    if last.as_ref() != Some(&s) {
                        if let Err(e) = app.emit(PLAYER_STATE_EVENT, &s) {
                            eprintln!("emit failed: {e}");
                        }
                        last = Some(s);
                    }
                }
                std::thread::sleep(std::time::Duration::from_millis(500));
            }
        })
        .expect("failed to spawn state emitter");
}
