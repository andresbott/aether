#[derive(Debug, Default)]
pub struct Queue {
    tracks: Vec<String>,
    index: Option<usize>,
}

impl Queue {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn set(&mut self, tracks: Vec<String>) {
        self.index = if tracks.is_empty() { None } else { Some(0) };
        self.tracks = tracks;
    }

    pub fn current(&self) -> Option<&str> {
        self.index.map(|i| self.tracks[i].as_str())
    }

    #[allow(clippy::should_implement_trait)]
    pub fn next(&mut self) -> Option<&str> {
        match self.index {
            Some(i) if i + 1 < self.tracks.len() => {
                self.index = Some(i + 1);
                self.current()
            }
            _ => None,
        }
    }

    #[allow(clippy::should_implement_trait)]
    pub fn prev(&mut self) -> Option<&str> {
        match self.index {
            Some(i) if i > 0 => {
                self.index = Some(i - 1);
                self.current()
            }
            _ => None,
        }
    }

    pub fn tracks(&self) -> &[String] {
        &self.tracks
    }

    pub fn index(&self) -> Option<usize> {
        self.index
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn q3() -> Queue {
        let mut q = Queue::new();
        q.set(vec!["a".into(), "b".into(), "c".into()]);
        q
    }

    #[test]
    fn set_positions_on_first_track() {
        let q = q3();
        assert_eq!(q.current(), Some("a"));
        assert_eq!(q.index(), Some(0));
    }

    #[test]
    fn set_empty_clears() {
        let mut q = q3();
        q.set(vec![]);
        assert_eq!(q.current(), None);
        assert_eq!(q.index(), None);
    }

    #[test]
    fn next_advances_and_stops_at_end() {
        let mut q = q3();
        assert_eq!(q.next(), Some("b"));
        assert_eq!(q.next(), Some("c"));
        assert_eq!(q.next(), None);
        assert_eq!(
            q.current(),
            Some("c"),
            "index unchanged after hitting the end"
        );
    }

    #[test]
    fn prev_goes_back_and_stops_at_start() {
        let mut q = q3();
        q.next();
        assert_eq!(q.prev(), Some("a"));
        assert_eq!(q.prev(), None);
        assert_eq!(q.current(), Some("a"));
    }

    #[test]
    fn next_on_empty_is_none() {
        let mut q = Queue::new();
        assert_eq!(q.next(), None);
        assert_eq!(q.prev(), None);
    }
}
