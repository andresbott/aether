//! System tray: closing the window hides it while playback continues in the
//! backend; the tray icon restores the UI or quits for real.
//! Desktop only — mobile has no tray.
//!
//! Two implementations:
//! - Windows/macOS: Tauri's built-in tray (left-click toggles, right-click menu).
//! - Linux: ksni (StatusNotifierItem). Tauri's tray goes through
//!   libappindicator, which is menu-only (no activate callback); ksni
//!   implements SNI directly so KDE/GNOME deliver left-click as `activate`.
use crate::state::AppState;
use tauri::{AppHandle, Manager};

fn toggle_main_window(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        if window.is_visible().unwrap_or(false) {
            let _ = window.hide();
        } else {
            let _ = window.unminimize();
            let _ = window.show();
            let _ = window.set_focus();
        }
    }
}

fn toggle_play_pause(app: &AppHandle) {
    let state = app.state::<AppState>();
    if state.player.state().paused {
        state.player.resume();
    } else {
        state.player.pause();
    }
}

fn next_track(app: &AppHandle) {
    let state = app.state::<AppState>();
    // load blocks on download; keep the tray menu responsive
    let player = state.player.clone();
    tauri::async_runtime::spawn_blocking(move || {
        if let Err(e) = player.queue_next() {
            eprintln!("tray: next track failed: {e}");
        }
    });
}

#[cfg(not(target_os = "linux"))]
pub fn setup(app: &AppHandle) -> tauri::Result<()> {
    use tauri::menu::{MenuBuilder, MenuItemBuilder};
    use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};

    let show = MenuItemBuilder::with_id("show", "Show / Hide").build(app)?;
    let play_pause = MenuItemBuilder::with_id("play_pause", "Play / Pause").build(app)?;
    let next = MenuItemBuilder::with_id("next", "Next Track").build(app)?;
    let quit = MenuItemBuilder::with_id("quit", "Quit").build(app)?;
    let menu = MenuBuilder::new(app)
        .items(&[&show, &play_pause, &next])
        .separator()
        .item(&quit)
        .build()?;

    let mut builder = TrayIconBuilder::with_id("main")
        .menu(&menu)
        // Menu only on right-click; left-click toggles the window instead.
        .show_menu_on_left_click(false)
        .tooltip("Aether Player")
        .on_menu_event(|app, event| match event.id().as_ref() {
            "show" => toggle_main_window(app),
            "play_pause" => toggle_play_pause(app),
            "next" => next_track(app),
            "quit" => app.exit(0),
            _ => {}
        })
        .on_tray_icon_event(|tray, event| {
            if let TrayIconEvent::Click {
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = event
            {
                toggle_main_window(tray.app_handle());
            }
        });
    if let Some(icon) = app.default_window_icon() {
        builder = builder.icon(icon.clone());
    }
    builder.build(app)?;
    Ok(())
}

#[cfg(target_os = "linux")]
pub fn setup(app: &AppHandle) -> tauri::Result<()> {
    use ksni::blocking::TrayMethods;
    use ksni::menu::{MenuItem, StandardItem};

    struct AetherTray {
        app: AppHandle,
        icon: Vec<ksni::Icon>,
    }

    impl ksni::Tray for AetherTray {
        fn id(&self) -> String {
            "dev.andresbott.aether.player".into()
        }

        fn title(&self) -> String {
            "Aether Player".into()
        }

        fn icon_pixmap(&self) -> Vec<ksni::Icon> {
            self.icon.clone()
        }

        // Left-click on the tray icon (SNI Activate).
        fn activate(&mut self, _x: i32, _y: i32) {
            toggle_main_window(&self.app);
        }

        fn menu(&self) -> Vec<MenuItem<Self>> {
            vec![
                StandardItem {
                    label: "Show / Hide".into(),
                    activate: Box::new(|tray: &mut Self| toggle_main_window(&tray.app)),
                    ..Default::default()
                }
                .into(),
                StandardItem {
                    label: "Play / Pause".into(),
                    activate: Box::new(|tray: &mut Self| toggle_play_pause(&tray.app)),
                    ..Default::default()
                }
                .into(),
                StandardItem {
                    label: "Next Track".into(),
                    activate: Box::new(|tray: &mut Self| next_track(&tray.app)),
                    ..Default::default()
                }
                .into(),
                MenuItem::Separator,
                StandardItem {
                    label: "Quit".into(),
                    activate: Box::new(|tray: &mut Self| tray.app.exit(0)),
                    ..Default::default()
                }
                .into(),
            ]
        }
    }

    // SNI wants ARGB32; Tauri's window icon is RGBA.
    let icon = app
        .default_window_icon()
        .map(|img| {
            let argb: Vec<u8> = img
                .rgba()
                .as_chunks::<4>()
                .0
                .iter()
                .flat_map(|&[r, g, b, a]| [a, r, g, b])
                .collect();
            vec![ksni::Icon {
                width: img.width() as i32,
                height: img.height() as i32,
                data: argb,
            }]
        })
        .unwrap_or_default();

    let tray = AetherTray {
        app: app.clone(),
        icon,
    };
    // No SNI host (rare, e.g. bare GNOME without the appindicator
    // extension) must not take the app down — run without a tray icon.
    if let Err(e) = tray.spawn() {
        eprintln!("tray: failed to start StatusNotifierItem service: {e}");
    }
    Ok(())
}
