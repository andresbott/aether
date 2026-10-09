//! Desktop playback engine: an actor thread owning a rodio Sink.
//! v0 strategy: download the full track (Subsonic `stream` endpoint), then
//! decode from memory. Progressive streaming is a follow-up.
use aether_core::error::CoreError;
use aether_core::player::PlaybackEngine;
use rodio::Decoder;
use std::io::Cursor;
use std::sync::mpsc::{self, Sender};
use std::sync::{Arc, Mutex};
use std::time::Duration;

enum Cmd {
    Load {
        url: String,
        reply: Sender<Result<(), String>>,
    },
    Pause,
    Resume,
    Stop,
    Seek(Duration),
    SetVolume(f32),
}

#[derive(Default)]
struct Shared {
    position: Duration,
    idle: bool,
}

pub struct RodioEngine {
    tx: Sender<Cmd>,
    shared: Arc<Mutex<Shared>>,
}

impl RodioEngine {
    pub fn start() -> Result<Self, CoreError> {
        let (tx, rx) = mpsc::channel::<Cmd>();
        let shared = Arc::new(Mutex::new(Shared {
            position: Duration::ZERO,
            idle: true,
        }));
        let shared_thread = shared.clone();
        // Fail fast if there is no audio device: open the stream on the
        // calling thread and move it into the actor.
        let (ready_tx, ready_rx) = mpsc::channel::<Result<(), String>>();

        std::thread::Builder::new()
            .name("aether-audio".into())
            .spawn(move || {
                let stream = match rodio::OutputStreamBuilder::open_default_stream() {
                    Ok(s) => {
                        let _ = ready_tx.send(Ok(()));
                        s
                    }
                    Err(e) => {
                        let _ = ready_tx.send(Err(e.to_string()));
                        return;
                    }
                };
                let player = rodio::Sink::connect_new(stream.mixer());
                loop {
                    match rx.recv_timeout(Duration::from_millis(250)) {
                        Ok(Cmd::Load { url, reply }) => {
                            let result = Self::do_load(&player, &url);
                            let _ = reply.send(result);
                        }
                        Ok(Cmd::Pause) => player.pause(),
                        Ok(Cmd::Resume) => player.play(),
                        Ok(Cmd::Stop) => player.stop(),
                        Ok(Cmd::Seek(pos)) => {
                            if let Err(e) = player.try_seek(pos) {
                                eprintln!("seek failed: {e}");
                            }
                        }
                        Ok(Cmd::SetVolume(v)) => player.set_volume(v),
                        Err(mpsc::RecvTimeoutError::Timeout) => {}
                        Err(mpsc::RecvTimeoutError::Disconnected) => return,
                    }
                    let mut s = shared_thread.lock().unwrap();
                    s.position = player.get_pos();
                    s.idle = player.empty();
                }
            })
            .map_err(|e| CoreError::Engine(format!("failed to spawn audio thread: {e}")))?;

        ready_rx
            .recv()
            .map_err(|_| CoreError::Engine("audio thread died".into()))?
            .map_err(CoreError::Engine)?;

        Ok(Self { tx, shared })
    }

    fn do_load(player: &rodio::Sink, url: &str) -> Result<(), String> {
        let bytes = reqwest::blocking::get(url)
            .and_then(|r| r.error_for_status())
            .and_then(|r| r.bytes())
            .map_err(|e| format!("download failed: {e}"))?;
        let source =
            Decoder::try_from(Cursor::new(bytes.to_vec())).map_err(|e| format!("decode: {e}"))?;
        player.stop();
        player.append(source);
        player.play();
        Ok(())
    }

    fn send(&self, cmd: Cmd) {
        if self.tx.send(cmd).is_err() {
            eprintln!("audio thread is gone");
        }
    }
}

impl PlaybackEngine for RodioEngine {
    fn load(&self, url: &str) -> Result<(), CoreError> {
        let (reply_tx, reply_rx) = mpsc::channel();
        self.send(Cmd::Load {
            url: url.to_string(),
            reply: reply_tx,
        });
        reply_rx
            .recv()
            .map_err(|_| CoreError::Engine("audio thread died".into()))?
            .map_err(CoreError::Decode)
    }
    fn pause(&self) {
        self.send(Cmd::Pause);
    }
    fn resume(&self) {
        self.send(Cmd::Resume);
    }
    fn stop(&self) {
        self.send(Cmd::Stop);
    }
    fn seek(&self, pos: Duration) -> Result<(), CoreError> {
        self.send(Cmd::Seek(pos));
        Ok(())
    }
    fn set_volume(&self, volume: f32) {
        self.send(Cmd::SetVolume(volume));
    }
    fn position(&self) -> Duration {
        self.shared.lock().unwrap().position
    }
    fn is_idle(&self) -> bool {
        self.shared.lock().unwrap().idle
    }
}
