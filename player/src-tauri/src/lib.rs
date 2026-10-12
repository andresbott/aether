mod commands;
mod credentials;
mod engine;
mod error;
mod events;
mod protocol;
mod settings;
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
        .plugin(tauri_plugin_http::init())
        .register_asynchronous_uri_scheme_protocol("aether", protocol::handle)
        .setup(|app| {
            let handle = app.handle();
            let state = AppState::new(settings::load(handle), credentials::load(handle))?;
            app.manage(state);
            events::spawn_state_emitter(app.handle().clone());
            #[cfg(desktop)]
            tray::setup(app.handle())?;
            Ok(())
        })
        .on_window_event(|window, event| {
            // Close-to-tray: hide the window instead of exiting. The webview is
            // not destroyed, so playback keeps running in it. Quit via the tray menu.
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
            commands::connection::clear_session,
            commands::connection::connection_status,
            commands::connection::set_session,
            commands::credentials::clear_api_key,
            commands::credentials::get_api_key,
            commands::credentials::set_api_key,
            commands::settings::clear_server_url,
            commands::settings::get_settings,
            commands::settings::set_server_url,
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
