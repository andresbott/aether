use crate::error::CoreError;
use crate::player::engine::{PlaybackEngine, StreamUrlResolver};
use crate::player::queue::Queue;
use crate::player::state::PlayerState;
use std::sync::{Arc, Mutex};
use std::time::Duration;

/// A media player that queues and plays tracks via a `PlaybackEngine`.
///
/// All engine calls are made while holding the player's internal lock. See the
/// `PlaybackEngine` documentation for the resulting concurrency contract.
pub struct Player {
    engine: Arc<dyn PlaybackEngine>,
    resolver: Arc<dyn StreamUrlResolver>,
    inner: Mutex<Inner>,
}

struct Inner {
    queue: Queue,
    paused: bool,
    stopped: bool,
    volume: f32,
}

impl Player {
    pub fn new(engine: Arc<dyn PlaybackEngine>, resolver: Arc<dyn StreamUrlResolver>) -> Self {
        Self {
            engine,
            resolver,
            inner: Mutex::new(Inner {
                queue: Queue::new(),
                paused: false,
                stopped: true,
                volume: 1.0,
            }),
        }
    }

    fn load_current(&self, inner: &mut Inner) -> Result<(), CoreError> {
        let Some(id) = inner.queue.current().map(str::to_string) else {
            inner.stopped = true;
            return Ok(());
        };
        let url = self.resolver.stream_url(&id)?;
        self.engine.load(&url)?;
        inner.paused = false;
        inner.stopped = false;
        Ok(())
    }

    pub fn play_track(&self, track_id: &str) -> Result<(), CoreError> {
        let mut inner = self.inner.lock().unwrap();
        inner.queue.set(vec![track_id.to_string()]);
        self.load_current(&mut inner)
    }

    pub fn queue_set(&self, track_ids: Vec<String>) -> Result<(), CoreError> {
        let mut inner = self.inner.lock().unwrap();
        inner.queue.set(track_ids);
        self.load_current(&mut inner)
    }

    pub fn queue_next(&self) -> Result<(), CoreError> {
        let mut inner = self.inner.lock().unwrap();
        if inner.queue.next().is_some() {
            self.load_current(&mut inner)
        } else {
            self.engine.stop();
            inner.stopped = true;
            Ok(())
        }
    }

    pub fn queue_prev(&self) -> Result<(), CoreError> {
        let mut inner = self.inner.lock().unwrap();
        if inner.queue.prev().is_some() {
            self.load_current(&mut inner)
        } else {
            Ok(())
        }
    }

    pub fn pause(&self) {
        self.engine.pause();
        self.inner.lock().unwrap().paused = true;
    }

    pub fn resume(&self) {
        self.engine.resume();
        self.inner.lock().unwrap().paused = false;
    }

    pub fn stop(&self) {
        self.engine.stop();
        let mut inner = self.inner.lock().unwrap();
        inner.stopped = true;
        inner.paused = false;
    }

    pub fn seek(&self, pos: Duration) -> Result<(), CoreError> {
        self.engine.seek(pos)
    }

    pub fn set_volume(&self, volume: f32) {
        let v = volume.clamp(0.0, 1.0);
        self.engine.set_volume(v);
        self.inner.lock().unwrap().volume = v;
    }

    pub fn state(&self) -> PlayerState {
        let inner = self.inner.lock().unwrap();
        PlayerState {
            track_id: inner.queue.current().map(str::to_string),
            position_ms: self.engine.position().as_millis() as u64,
            paused: inner.paused,
            stopped: inner.stopped,
            volume: inner.volume,
            queue: inner.queue.tracks().to_vec(),
            queue_index: inner.queue.index(),
        }
    }

    /// Call periodically (e.g. every 500ms). Advances the queue when the
    /// engine reports the current track finished.
    pub fn poll(&self) {
        let mut inner = self.inner.lock().unwrap();
        if inner.stopped || inner.paused || !self.engine.is_idle() {
            return;
        }
        if inner.queue.next().is_some() {
            if let Err(e) = self.load_current(&mut inner) {
                eprintln!("auto-advance failed: {e}");
                inner.stopped = true;
            }
        } else {
            inner.stopped = true;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicBool, Ordering};
    use std::sync::Mutex;

    #[derive(Default)]
    struct FakeEngine {
        loads: Mutex<Vec<String>>,
        idle: AtomicBool,
        paused: AtomicBool,
    }

    impl PlaybackEngine for FakeEngine {
        fn load(&self, url: &str) -> Result<(), CoreError> {
            self.loads.lock().unwrap().push(url.to_string());
            self.idle.store(false, Ordering::SeqCst);
            Ok(())
        }
        fn pause(&self) {
            self.paused.store(true, Ordering::SeqCst);
        }
        fn resume(&self) {
            self.paused.store(false, Ordering::SeqCst);
        }
        fn stop(&self) {
            self.idle.store(true, Ordering::SeqCst);
        }
        fn seek(&self, _pos: Duration) -> Result<(), CoreError> {
            Ok(())
        }
        fn set_volume(&self, _volume: f32) {}
        fn position(&self) -> Duration {
            Duration::from_secs(1)
        }
        fn is_idle(&self) -> bool {
            self.idle.load(Ordering::SeqCst)
        }
    }

    struct FakeResolver;
    impl StreamUrlResolver for FakeResolver {
        fn stream_url(&self, track_id: &str) -> Result<String, CoreError> {
            Ok(format!("http://x/stream?id={track_id}"))
        }
    }

    fn player() -> (Arc<FakeEngine>, Player) {
        let engine = Arc::new(FakeEngine::default());
        engine.idle.store(true, Ordering::SeqCst);
        let p = Player::new(engine.clone(), Arc::new(FakeResolver));
        (engine, p)
    }

    #[test]
    fn play_track_loads_resolved_url_and_sets_single_queue() {
        let (engine, p) = player();
        p.play_track("t1").unwrap();
        assert_eq!(engine.loads.lock().unwrap()[0], "http://x/stream?id=t1");
        let s = p.state();
        assert_eq!(s.queue, vec!["t1".to_string()]);
        assert_eq!(s.track_id.as_deref(), Some("t1"));
        assert!(!s.stopped && !s.paused);
    }

    #[test]
    fn queue_set_plays_first_track() {
        let (engine, p) = player();
        p.queue_set(vec!["a".into(), "b".into()]).unwrap();
        assert_eq!(engine.loads.lock().unwrap().len(), 1);
        assert_eq!(p.state().queue_index, Some(0));
    }

    #[test]
    fn queue_next_loads_next_and_stops_at_end() {
        let (engine, p) = player();
        p.queue_set(vec!["a".into(), "b".into()]).unwrap();
        p.queue_next().unwrap();
        assert_eq!(engine.loads.lock().unwrap()[1], "http://x/stream?id=b");
        p.queue_next().unwrap(); // at end: stops
        assert!(p.state().stopped);
    }

    #[test]
    fn poll_auto_advances_when_track_ends() {
        let (engine, p) = player();
        p.queue_set(vec!["a".into(), "b".into()]).unwrap();
        engine.idle.store(true, Ordering::SeqCst); // simulate track end
        p.poll();
        assert_eq!(engine.loads.lock().unwrap()[1], "http://x/stream?id=b");
        // end of queue + idle again -> stopped, no more loads
        engine.idle.store(true, Ordering::SeqCst);
        p.poll();
        assert!(p.state().stopped);
        assert_eq!(engine.loads.lock().unwrap().len(), 2);
    }

    #[test]
    fn pause_resume_reflected_in_state() {
        let (_e, p) = player();
        p.play_track("t1").unwrap();
        p.pause();
        assert!(p.state().paused);
        p.resume();
        assert!(!p.state().paused);
    }

    #[test]
    fn stop_marks_stopped_and_poll_does_not_advance() {
        let (engine, p) = player();
        p.queue_set(vec!["a".into(), "b".into()]).unwrap();
        p.stop();
        p.poll();
        assert!(p.state().stopped);
        assert_eq!(engine.loads.lock().unwrap().len(), 1);
    }

    #[test]
    fn set_volume_clamps_and_shows_in_state() {
        let (_e, p) = player();
        p.set_volume(1.7);
        assert_eq!(p.state().volume, 1.0);
        p.set_volume(-0.2);
        assert_eq!(p.state().volume, 0.0);
    }
}
