mod commands;
mod cookie_jar;
mod credentials;
mod engine;
mod error;
mod events;
mod protocol;
mod session;
mod settings;
mod state;
#[cfg(desktop)]
mod tray;

use session::Session;
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
            // Without a config directory nothing loads and every save fails, but
            // the app still starts: the login view asks for a server.
            let config_dir = match app.path().app_config_dir() {
                Ok(dir) => Some(dir),
                Err(e) => {
                    eprintln!("session: no config directory, nothing will be saved: {e}");
                    None
                }
            };
            let state = AppState::new(Session::load(config_dir))?;
            app.manage(state);
            // The plugin's setup ran in build(): its cookie jar exists by now, and
            // no response has saved the session cookie in it yet.
            if let Ok(cache_dir) = app.path().app_cache_dir() {
                if let Err(e) = cookie_jar::restrict_to_owner(&cache_dir) {
                    eprintln!("cookie jar: cannot make it owner-only: {e}");
                }
            }
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
            commands::settings::set_session_kind,
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
