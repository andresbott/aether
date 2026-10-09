use crate::error::CoreError;
use rand::{distributions::Alphanumeric, Rng};
use serde::Deserialize;
use url::{ParseError, Url};

pub const API_VERSION: &str = "1.16.1";
pub const CLIENT_NAME: &str = "aether-player";

/// How requests prove who is asking. The two parameter sets are never sent
/// together: the server answers a mix with error 43.
#[derive(Clone)]
enum Auth {
    Password { username: String, password: String },
    ApiKey(String),
}

#[derive(Clone)]
pub struct SubsonicClient {
    http: reqwest::Client,
    base_url: Url,
    auth: Auth,
}

#[derive(Debug, Deserialize)]
struct Envelope {
    #[serde(rename = "subsonic-response")]
    subsonic_response: ResponseBody,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct ResponseBody {
    pub status: String,
    pub error: Option<ApiError>,
    pub artists: Option<ArtistsIndex>,
    pub album_list2: Option<AlbumList2>,
    pub album: Option<super::models::Album>,
    pub search_result3: Option<super::models::SearchResult>,
}

#[derive(Debug, Deserialize)]
pub(crate) struct ApiError {
    pub code: u32,
    pub message: String,
}

#[derive(Debug, Deserialize)]
pub(crate) struct ArtistsIndex {
    #[serde(default)]
    pub index: Vec<IndexEntry>,
}

#[derive(Debug, Deserialize)]
pub(crate) struct IndexEntry {
    #[serde(default)]
    pub artist: Vec<super::models::Artist>,
}

#[derive(Debug, Deserialize)]
pub(crate) struct AlbumList2 {
    #[serde(default)]
    pub album: Vec<super::models::Album>,
}

/// Checks a server address: an `http` or `https` URL with a host and without a
/// query or fragment (the client adds its own to every request). A missing host
/// (`http://`) is refused by the URL parser itself.
fn parse_server_url(input: &str) -> Result<Url, CoreError> {
    let url = Url::parse(input).map_err(|e| {
        CoreError::InvalidServerUrl(match e {
            ParseError::RelativeUrlWithoutBase => {
                "missing scheme, expected http:// or https://".into()
            }
            other => other.to_string(),
        })
    })?;
    if !matches!(url.scheme(), "http" | "https") {
        return Err(CoreError::InvalidServerUrl(format!(
            "unsupported scheme {:?}, expected http or https",
            url.scheme()
        )));
    }
    if url.query().is_some() {
        return Err(CoreError::InvalidServerUrl(
            "must not contain a query".into(),
        ));
    }
    if url.fragment().is_some() {
        return Err(CoreError::InvalidServerUrl(
            "must not contain a fragment".into(),
        ));
    }
    Ok(url)
}

/// The text of a validated server URL, without trailing slashes.
fn canonical(url: &Url) -> String {
    url.as_str().trim_end_matches('/').to_string()
}

/// Checks a server address and returns its canonical form: an `http` or
/// `https` URL with a host, no query or fragment and no trailing slash
/// (`http://h:8075/` becomes `http://h:8075`). The port and a path prefix, for
/// a server mounted behind a reverse proxy, are kept.
///
/// [`SubsonicClient`] applies the same checks to the address it is built with.
pub fn normalize_server_url(input: &str) -> Result<String, CoreError> {
    parse_server_url(input).map(|url| canonical(&url))
}

impl SubsonicClient {
    /// A client that signs requests with the salted-token scheme (`u`/`t`/`s`).
    pub fn new(base_url: &str, username: &str, password: &str) -> Result<Self, CoreError> {
        Self::with_auth(
            base_url,
            Auth::Password {
                username: username.to_string(),
                password: password.to_string(),
            },
        )
    }

    /// A client that sends an OpenSubsonic API key (`apiKey`) instead of a
    /// username and password.
    pub fn with_api_key(base_url: &str, api_key: &str) -> Result<Self, CoreError> {
        Self::with_auth(base_url, Auth::ApiKey(api_key.to_string()))
    }

    fn with_auth(base_url: &str, auth: Auth) -> Result<Self, CoreError> {
        Ok(Self {
            http: reqwest::Client::new(),
            base_url: parse_server_url(base_url)?,
            auth,
        })
    }

    /// The credentials followed by the fixed client parameters. Password auth
    /// uses a fresh salt + token per call, per the Subsonic auth scheme; an
    /// API key is sent as is.
    fn auth_query(&self) -> Vec<(String, String)> {
        let mut query = match &self.auth {
            Auth::Password { username, password } => {
                let salt: String = rand::thread_rng()
                    .sample_iter(&Alphanumeric)
                    .take(12)
                    .map(char::from)
                    .collect();
                let token = format!("{:x}", md5::compute(format!("{password}{salt}")));
                vec![
                    ("u".into(), username.clone()),
                    ("t".into(), token),
                    ("s".into(), salt),
                ]
            }
            Auth::ApiKey(key) => vec![("apiKey".into(), key.clone())],
        };
        query.extend([
            ("v".into(), API_VERSION.into()),
            ("c".into(), CLIENT_NAME.into()),
            ("f".into(), "json".into()),
        ]);
        query
    }

    pub(crate) fn build_url(&self, path: &str, params: &[(&str, &str)]) -> Url {
        let mut url = self.base_url.clone();
        url.set_path(&format!(
            "{}/{}",
            self.base_url.path().trim_end_matches('/'),
            path.trim_start_matches('/')
        ));
        {
            let mut q = url.query_pairs_mut();
            for (k, v) in self.auth_query() {
                q.append_pair(&k, &v);
            }
            for (k, v) in params {
                q.append_pair(k, v);
            }
        }
        url
    }

    pub(crate) async fn get(
        &self,
        path: &str,
        params: &[(&str, &str)],
    ) -> Result<ResponseBody, CoreError> {
        let url = self.build_url(path, params);
        let resp = self
            .http
            .get(url)
            .send()
            .await
            .map_err(|e| CoreError::ServerUnreachable(e.to_string()))?;
        let envelope: Envelope = resp
            .json()
            .await
            .map_err(|e| CoreError::InvalidResponse(e.to_string()))?;
        let body = envelope.subsonic_response;
        if body.status != "ok" {
            return match body.error {
                // 40: wrong or missing credentials, 44: invalid apiKey
                Some(ApiError { code: 40 | 44, .. }) => Err(CoreError::AuthFailed),
                Some(ApiError { code, message }) => Err(CoreError::Api { code, message }),
                None => Err(CoreError::InvalidResponse("failed without error".into())),
            };
        }
        Ok(body)
    }

    pub async fn ping(&self) -> Result<(), CoreError> {
        self.get("rest/ping", &[]).await.map(|_| ())
    }

    pub async fn get_artists(&self) -> Result<Vec<super::models::Artist>, CoreError> {
        let body = self.get("rest/getArtists", &[]).await?;
        let index = body
            .artists
            .ok_or_else(|| CoreError::InvalidResponse("missing artists".into()))?;
        Ok(index.index.into_iter().flat_map(|i| i.artist).collect())
    }

    pub async fn get_albums(
        &self,
        list_type: &str,
        size: u32,
        offset: u32,
    ) -> Result<Vec<super::models::Album>, CoreError> {
        let size = size.to_string();
        let offset = offset.to_string();
        let body = self
            .get(
                "rest/getAlbumList2",
                &[("type", list_type), ("size", &size), ("offset", &offset)],
            )
            .await?;
        let list = body
            .album_list2
            .ok_or_else(|| CoreError::InvalidResponse("missing albumList2".into()))?;
        Ok(list.album)
    }

    pub async fn get_album(&self, id: &str) -> Result<super::models::Album, CoreError> {
        let body = self.get("rest/getAlbum", &[("id", id)]).await?;
        body.album
            .ok_or_else(|| CoreError::InvalidResponse("missing album".into()))
    }

    pub async fn search(&self, query: &str) -> Result<super::models::SearchResult, CoreError> {
        let body = self.get("rest/search3", &[("query", query)]).await?;
        Ok(body.search_result3.unwrap_or_default())
    }

    pub fn stream_url(&self, track_id: &str) -> String {
        self.build_url("rest/stream", &[("id", track_id)]).into()
    }

    pub fn cover_art_url(&self, cover_id: &str, size: Option<u32>) -> String {
        match size {
            Some(s) => self
                .build_url(
                    "rest/getCoverArt",
                    &[("id", cover_id), ("size", &s.to_string())],
                )
                .into(),
            None => self
                .build_url("rest/getCoverArt", &[("id", cover_id)])
                .into(),
        }
    }

    /// The server address in the form [`normalize_server_url`] returns.
    pub fn base_url_str(&self) -> String {
        canonical(&self.base_url)
    }

    // Transitional: Task 2 removes this together with its only caller.
    pub fn username_str(&self) -> String {
        match &self.auth {
            Auth::Password { username, .. } => username.clone(),
            Auth::ApiKey(_) => String::new(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    fn ok_body(extra: &str) -> String {
        format!(
            r#"{{"subsonic-response":{{"status":"ok","version":"1.16.1"{}}}}}"#,
            extra
        )
    }

    #[tokio::test]
    async fn ping_sends_auth_params_and_succeeds() {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/rest/ping"))
            .respond_with(ResponseTemplate::new(200).set_body_string(ok_body("")))
            .mount(&server)
            .await;

        let client = SubsonicClient::new(&server.uri(), "admin", "sesame").unwrap();
        client.ping().await.unwrap();

        let reqs = server.received_requests().await.unwrap();
        let q: std::collections::HashMap<String, String> = reqs[0]
            .url
            .query_pairs()
            .map(|(k, v)| (k.into_owned(), v.into_owned()))
            .collect();
        assert_eq!(q["u"], "admin");
        assert_eq!(q["v"], "1.16.1");
        assert_eq!(q["c"], "aether-player");
        assert_eq!(q["f"], "json");
        let expected = format!("{:x}", md5::compute(format!("sesame{}", q["s"])));
        assert_eq!(q["t"], expected, "token must be md5(password + salt)");
        assert!(
            !q.contains_key("apiKey"),
            "password auth must not send an apiKey"
        );
    }

    #[tokio::test]
    async fn ping_with_api_key_sends_only_the_key() {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/rest/ping"))
            .respond_with(ResponseTemplate::new(200).set_body_string(ok_body("")))
            .mount(&server)
            .await;

        let client = SubsonicClient::with_api_key(&server.uri(), "s3cr3t").unwrap();
        client.ping().await.unwrap();

        let reqs = server.received_requests().await.unwrap();
        let q: std::collections::HashMap<String, String> = reqs[0]
            .url
            .query_pairs()
            .map(|(k, v)| (k.into_owned(), v.into_owned()))
            .collect();
        assert_eq!(q["apiKey"], "s3cr3t");
        // the server answers a key mixed with password params with error 43
        for password_param in ["u", "t", "s"] {
            assert!(
                !q.contains_key(password_param),
                "api key auth must not send `{password_param}`"
            );
        }
        assert_eq!(q["v"], "1.16.1");
        assert_eq!(q["c"], "aether-player");
        assert_eq!(q["f"], "json");
    }

    #[tokio::test]
    async fn ping_maps_error_44_to_auth_failed() {
        let server = MockServer::start().await;
        let body = r#"{"subsonic-response":{"status":"failed","version":"1.16.1",
            "error":{"code":44,"message":"Invalid API key"}}}"#;
        Mock::given(method("GET"))
            .and(path("/rest/ping"))
            .respond_with(ResponseTemplate::new(200).set_body_string(body))
            .mount(&server)
            .await;

        let client = SubsonicClient::with_api_key(&server.uri(), "revoked").unwrap();
        let err = client.ping().await.unwrap_err();
        assert!(matches!(err, CoreError::AuthFailed));
    }

    #[tokio::test]
    async fn ping_maps_error_40_to_auth_failed() {
        let server = MockServer::start().await;
        let body = r#"{"subsonic-response":{"status":"failed","version":"1.16.1",
            "error":{"code":40,"message":"Wrong username or password"}}}"#;
        Mock::given(method("GET"))
            .and(path("/rest/ping"))
            .respond_with(ResponseTemplate::new(200).set_body_string(body))
            .mount(&server)
            .await;

        let client = SubsonicClient::new(&server.uri(), "admin", "wrong").unwrap();
        let err = client.ping().await.unwrap_err();
        assert!(matches!(err, CoreError::AuthFailed));
    }

    #[tokio::test]
    async fn ping_unreachable_server_maps_to_server_unreachable() {
        // port 1 is never listening
        let client = SubsonicClient::new("http://127.0.0.1:1", "u", "p").unwrap();
        let err = client.ping().await.unwrap_err();
        assert!(matches!(err, CoreError::ServerUnreachable(_)));
    }

    #[tokio::test]
    async fn get_artists_flattens_indexes() {
        let server = MockServer::start().await;
        let extra = r#","artists":{"index":[
            {"name":"A","artist":[{"id":"ar1","name":"Alpha","albumCount":2}]},
            {"name":"B","artist":[{"id":"ar2","name":"Beta"}]}]}"#;
        Mock::given(method("GET"))
            .and(path("/rest/getArtists"))
            .respond_with(ResponseTemplate::new(200).set_body_string(ok_body(extra)))
            .mount(&server)
            .await;

        let client = SubsonicClient::new(&server.uri(), "u", "p").unwrap();
        let artists = client.get_artists().await.unwrap();
        assert_eq!(artists.len(), 2);
        assert_eq!(artists[0].id, "ar1");
        assert_eq!(artists[1].name, "Beta");
    }

    #[tokio::test]
    async fn get_albums_sends_paging_params() {
        let server = MockServer::start().await;
        let extra = r#","albumList2":{"album":[{"id":"al1","name":"One","artist":"Alpha"}]}"#;
        Mock::given(method("GET"))
            .and(path("/rest/getAlbumList2"))
            .respond_with(ResponseTemplate::new(200).set_body_string(ok_body(extra)))
            .mount(&server)
            .await;

        let client = SubsonicClient::new(&server.uri(), "u", "p").unwrap();
        let albums = client
            .get_albums("alphabeticalByName", 20, 40)
            .await
            .unwrap();
        assert_eq!(albums.len(), 1);
        assert_eq!(albums[0].id, "al1");

        let reqs = server.received_requests().await.unwrap();
        let q: std::collections::HashMap<String, String> = reqs[0]
            .url
            .query_pairs()
            .map(|(k, v)| (k.into_owned(), v.into_owned()))
            .collect();
        assert_eq!(q["type"], "alphabeticalByName");
        assert_eq!(q["size"], "20");
        assert_eq!(q["offset"], "40");
    }

    #[tokio::test]
    async fn get_album_returns_album_with_songs() {
        let server = MockServer::start().await;
        let extra = r#","album":{"id":"al1","name":"One","songCount":1,
            "song":[{"id":"t1","title":"Intro","duration":10}]}"#;
        Mock::given(method("GET"))
            .and(path("/rest/getAlbum"))
            .respond_with(ResponseTemplate::new(200).set_body_string(ok_body(extra)))
            .mount(&server)
            .await;

        let client = SubsonicClient::new(&server.uri(), "u", "p").unwrap();
        let album = client.get_album("al1").await.unwrap();
        assert_eq!(album.song.len(), 1);
        assert_eq!(album.song[0].title, "Intro");
    }

    #[tokio::test]
    async fn search_returns_all_three_kinds() {
        let server = MockServer::start().await;
        let extra = r#","searchResult3":{
            "artist":[{"id":"ar1","name":"Alpha"}],
            "album":[{"id":"al1","name":"One"}],
            "song":[{"id":"t1","title":"Intro"}]}"#;
        Mock::given(method("GET"))
            .and(path("/rest/search3"))
            .respond_with(ResponseTemplate::new(200).set_body_string(ok_body(extra)))
            .mount(&server)
            .await;

        let client = SubsonicClient::new(&server.uri(), "u", "p").unwrap();
        let res = client.search("alp").await.unwrap();
        assert_eq!(res.artist.len(), 1);
        assert_eq!(res.album.len(), 1);
        assert_eq!(res.song.len(), 1);
    }

    #[test]
    fn stream_url_contains_id_and_auth() {
        let client = SubsonicClient::new("http://example.com:8075", "u", "p").unwrap();
        let url = url::Url::parse(&client.stream_url("t42")).unwrap();
        assert_eq!(url.path(), "/rest/stream");
        let q: std::collections::HashMap<_, _> = url.query_pairs().collect();
        assert_eq!(q["id"], "t42");
        assert!(q.contains_key("t") && q.contains_key("s") && q.contains_key("u"));
    }

    #[test]
    fn cover_art_url_includes_optional_size() {
        let client = SubsonicClient::new("http://example.com", "u", "p").unwrap();
        let url = url::Url::parse(&client.cover_art_url("c9", Some(300))).unwrap();
        assert_eq!(url.path(), "/rest/getCoverArt");
        let q: std::collections::HashMap<_, _> = url.query_pairs().collect();
        assert_eq!(q["id"], "c9");
        assert_eq!(q["size"], "300");
    }

    #[tokio::test]
    async fn requests_keep_the_server_path_prefix() {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/aether/rest/ping"))
            .respond_with(ResponseTemplate::new(200).set_body_string(ok_body("")))
            .mount(&server)
            .await;

        // the trailing slash must not turn into `//rest/ping`
        let base = format!("{}/aether/", server.uri());
        let client = SubsonicClient::with_api_key(&base, "k").unwrap();
        client.ping().await.unwrap();
    }

    #[test]
    fn constructors_reject_invalid_server_urls() {
        let err = SubsonicClient::new("ftp://example.com", "u", "p").err();
        assert!(matches!(err, Some(CoreError::InvalidServerUrl(_))));
        let err = SubsonicClient::with_api_key("example.com", "k").err();
        assert!(matches!(err, Some(CoreError::InvalidServerUrl(_))));
    }

    #[test]
    fn base_url_str_is_the_normalised_address() {
        for input in [
            "http://example.com:8075",
            "http://example.com:8075/",
            "https://example.com/aether/",
        ] {
            let client = SubsonicClient::new(input, "u", "p").unwrap();
            assert_eq!(client.base_url_str(), normalize_server_url(input).unwrap());
        }
    }

    // Goes away with `username_str` (Task 2).
    #[test]
    fn username_str_is_only_set_for_password_auth() {
        let client = SubsonicClient::new("http://example.com", "admin", "p").unwrap();
        assert_eq!(client.username_str(), "admin");
        let client = SubsonicClient::with_api_key("http://example.com", "k").unwrap();
        assert_eq!(client.username_str(), "");
    }

    /// Asserts that `input` normalises to `want`.
    fn assert_normalised(input: &str, want: &str) {
        let got = normalize_server_url(input)
            .unwrap_or_else(|e| panic!("{input:?} should be accepted: {e}"));
        assert_eq!(got, want, "input {input:?}");
    }

    /// Asserts that `input` is refused with `InvalidServerUrl`.
    fn assert_rejected(input: &str) {
        match normalize_server_url(input) {
            Err(CoreError::InvalidServerUrl(_)) => {}
            other => panic!("{input:?} should be an invalid server url, got {other:?}"),
        }
    }

    #[test]
    fn normalize_server_url_trims_trailing_slashes() {
        for (input, want) in [
            ("http://h:8075", "http://h:8075"),
            ("http://h:8075/", "http://h:8075"),
            ("https://music.example.com///", "https://music.example.com"),
            ("http://h/aether/", "http://h/aether"),
            // pasted addresses often carry stray whitespace
            ("  http://h:8075/\n", "http://h:8075"),
        ] {
            assert_normalised(input, want);
        }
    }

    #[test]
    fn normalize_server_url_keeps_port_and_path_prefix() {
        for (input, want) in [
            ("http://192.168.1.5:8075", "http://192.168.1.5:8075"),
            ("https://example.com:8443/", "https://example.com:8443"),
            ("http://h/aether", "http://h/aether"),
            (
                "https://example.com:8443/media/aether/",
                "https://example.com:8443/media/aether",
            ),
        ] {
            assert_normalised(input, want);
        }
    }

    #[test]
    fn normalize_server_url_rejects_other_schemes() {
        for input in ["ftp://h/music", "file:///srv/music", "ws://h:8075"] {
            assert_rejected(input);
        }
    }

    #[test]
    fn normalize_server_url_rejects_a_missing_scheme() {
        for input in [
            "",
            "aether.local",
            "192.168.1.5:8075",
            "localhost:8075",
            "//h:8075",
        ] {
            assert_rejected(input);
        }
    }

    #[test]
    fn normalize_server_url_rejects_a_missing_host() {
        for input in ["http://", "https:///", "http://:8075"] {
            assert_rejected(input);
        }
    }

    #[test]
    fn normalize_server_url_rejects_query_and_fragment() {
        for input in [
            "http://h:8075/?token=x",
            "http://h:8075?x=1",
            "http://h/aether/#top",
            "http://h/?",
            "http://h/#",
        ] {
            assert_rejected(input);
        }
    }
}
