use crate::state::AppState;
use tauri::{AppHandle, Emitter, Manager};

pub const PLAYER_STATE_EVENT: &str = "player://state";

/// The tray menu's Play / Pause and Next, sent to the webview. Playback lives in
/// the SPA's own player for now and the Rust player stays idle, so the tray asks
/// the webview to act (webui/src/lib/host/events.ts listens). The payload is one
/// of the two plain strings below.
pub const TRAY_COMMAND_EVENT: &str = "tray://command";
/// `TRAY_COMMAND_EVENT` payload: toggle play / pause.
pub const TRAY_PLAY_PAUSE: &str = "play_pause";
/// `TRAY_COMMAND_EVENT` payload: skip to the next track.
pub const TRAY_NEXT: &str = "next";

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

#[cfg(test)]
mod tests {
    use super::*;

    // The webview listens for these exact strings (webui/src/lib/host/events.ts);
    // a rename here breaks it silently, since nothing links the two at build time.
    #[test]
    fn event_names_and_tray_commands_are_the_ones_the_webview_listens_for() {
        assert_eq!(PLAYER_STATE_EVENT, "player://state");
        assert_eq!(TRAY_COMMAND_EVENT, "tray://command");
        assert_eq!(TRAY_PLAY_PAUSE, "play_pause");
        assert_eq!(TRAY_NEXT, "next");
    }
}
