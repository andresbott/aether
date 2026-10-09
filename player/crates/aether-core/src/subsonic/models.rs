use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct Artist {
    pub id: String,
    pub name: String,
    #[serde(default)]
    pub album_count: Option<u32>,
    #[serde(default)]
    pub cover_art: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct Album {
    pub id: String,
    pub name: String,
    #[serde(default)]
    pub artist: Option<String>,
    #[serde(default)]
    pub artist_id: Option<String>,
    #[serde(default)]
    pub song_count: Option<u32>,
    #[serde(default)]
    pub duration: Option<u32>,
    #[serde(default)]
    pub cover_art: Option<String>,
    #[serde(default)]
    pub song: Vec<Track>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct Track {
    pub id: String,
    pub title: String,
    #[serde(default)]
    pub album: Option<String>,
    #[serde(default)]
    pub album_id: Option<String>,
    #[serde(default)]
    pub artist: Option<String>,
    #[serde(default)]
    pub duration: Option<u32>,
    #[serde(default)]
    pub track: Option<u32>,
    #[serde(default)]
    pub cover_art: Option<String>,
    #[serde(default)]
    pub content_type: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Default)]
#[serde(rename_all = "camelCase")]
pub struct SearchResult {
    #[serde(default)]
    pub artist: Vec<Artist>,
    #[serde(default)]
    pub album: Vec<Album>,
    #[serde(default)]
    pub song: Vec<Track>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn track_deserializes_from_subsonic_json() {
        let json = r#"{"id":"t1","title":"Song","album":"Al","albumId":"a1",
            "artist":"Ar","duration":215,"track":3,"coverArt":"c1","contentType":"audio/mpeg"}"#;
        let t: Track = serde_json::from_str(json).unwrap();
        assert_eq!(t.id, "t1");
        assert_eq!(t.album_id.as_deref(), Some("a1"));
        assert_eq!(t.duration, Some(215));
        assert_eq!(t.cover_art.as_deref(), Some("c1"));
    }
}
