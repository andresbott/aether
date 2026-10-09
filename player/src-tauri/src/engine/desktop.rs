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
        let bytes = download(url)?;
        let source = Decoder::try_from(Cursor::new(bytes)).map_err(|e| format!("decode: {e}"))?;
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

/// Downloads the whole track at `url`.
///
/// The error text is built without the request URL. reqwest prints it for a
/// failed send as well as for a bad status, and a stream URL carries the API
/// key; the text reaches the UI (`AppError.message`) and stderr.
fn download(url: &str) -> Result<Vec<u8>, String> {
    reqwest::blocking::get(url)
        .and_then(|r| r.error_for_status())
        .and_then(|r| r.bytes())
        .map(|bytes| bytes.to_vec())
        .map_err(|e| format!("download failed: {}", e.without_url()))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{Read, Write};
    use std::net::TcpListener;

    /// Answers one request on a free loopback port with a canned response and
    /// returns the port. Plain std, so no mock-server crate and no audio device.
    fn serve_once(status: &str, body: &'static str) -> u16 {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        let status = status.to_string();
        std::thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            // read the whole request first: closing a socket with unread data
            // makes the peer see a reset instead of the response
            let mut request = Vec::new();
            let mut buf = [0u8; 512];
            while !request.windows(4).any(|w| w == b"\r\n\r\n") {
                let n = stream.read(&mut buf).unwrap();
                if n == 0 {
                    break;
                }
                request.extend_from_slice(&buf[..n]);
            }
            write!(
                stream,
                "HTTP/1.1 {status}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                body.len()
            )
            .unwrap();
        });
        port
    }

    #[test]
    fn download_returns_the_body() {
        let port = serve_once("200 OK", "track bytes");
        let url = format!("http://127.0.0.1:{port}/rest/stream?apiKey=SECRET");
        assert_eq!(download(&url).unwrap(), b"track bytes");
    }

    #[test]
    fn a_failed_send_does_not_leak_the_url() {
        // port 1 is never listening
        let err = download("http://127.0.0.1:1/rest/stream?apiKey=SECRET").unwrap_err();
        // the text reaches the UI (AppError.message) and stderr; reqwest would
        // print the request URL, which carries the API key
        assert!(
            !err.contains("SECRET"),
            "the api key leaked into the error: {err}"
        );
    }

    #[test]
    fn a_bad_status_does_not_leak_the_url() {
        let port = serve_once("404 Not Found", "");
        let url = format!("http://127.0.0.1:{port}/rest/stream?apiKey=SECRET");
        let err = download(&url).unwrap_err();
        // reaching the status check, not a connection failure, is the point
        assert!(
            err.contains("404"),
            "expected the status in the error: {err}"
        );
        assert!(
            !err.contains("SECRET"),
            "the api key leaked into the error: {err}"
        );
    }
}
