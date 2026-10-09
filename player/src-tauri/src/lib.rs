mod commands;
mod credentials;
mod engine;
mod error;
mod events;
mod protocol;
mod state;
#[cfg(desktop)]
mod tray;

use state::AppState;
use tauri::Manager;

/// The version the bundle carries: tauri.conf.json's `version`, which the
/// release workflow overrides with the git tag — not `CARGO_PKG_VERSION`,
/// which stays at the workspace's dev version (docs/agents/player/releasing.md).
#[tauri::command]
fn app_version(app: tauri::AppHandle) -> String {
    app.package_info().version.to_string()
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .register_asynchronous_uri_scheme_protocol("aether", protocol::handle)
        .setup(|app| {
            let state = AppState::new()?;
            app.manage(state);
            if let Some((stored, password)) = credentials::load(&app.handle().clone()) {
                match aether_core::subsonic::SubsonicClient::new(
                    &stored.server_url,
                    &stored.username,
                    &password,
                ) {
                    Ok(client) => {
                        let st = app.state::<AppState>();
                        *st.connection.write().unwrap() = Some(client);
                    }
                    Err(e) => {
                        eprintln!("failed to restore connection: {e}");
                    }
                }
            }
            events::spawn_state_emitter(app.handle().clone());
            #[cfg(desktop)]
            tray::setup(app.handle())?;
            Ok(())
        })
        .on_window_event(|window, event| {
            // Close-to-tray: hide the window instead of exiting so playback
            // keeps running in the backend. Quit via the tray menu.
            #[cfg(desktop)]
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
            }
            #[cfg(not(desktop))]
            let _ = (window, event);
        })
        .invoke_handler(tauri::generate_handler![
            app_version,
            commands::connection::connect,
            commands::connection::connection_status,
            commands::connection::disconnect,
            commands::library::get_artists,
            commands::library::get_albums,
            commands::library::get_album,
            commands::library::search,
            commands::playback::play_track,
            commands::playback::queue_set,
            commands::playback::queue_next,
            commands::playback::queue_prev,
            commands::playback::queue_get,
            commands::playback::pause,
            commands::playback::resume,
            commands::playback::stop,
            commands::playback::seek,
            commands::playback::set_volume,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
