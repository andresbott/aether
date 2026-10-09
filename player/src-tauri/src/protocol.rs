use crate::state::AppState;
use tauri::http::{Request, Response, StatusCode};
use tauri::{Manager, UriSchemeContext, UriSchemeResponder};

/// Builds an HTTP response with panic-free fallback.
/// If response building fails, logs the error and returns an empty 200 response.
fn build_response(
    status: StatusCode,
    content_type: Option<&str>,
    body: Vec<u8>,
) -> Response<Vec<u8>> {
    let mut builder = Response::builder().status(status);
    if let Some(ct) = content_type {
        builder = builder.header("content-type", ct);
    }
    builder.body(body).unwrap_or_else(|e| {
        eprintln!("aether protocol: failed to build response: {e}");
        Response::new(Vec::new())
    })
}

/// Handles aether://cover/<id>[?size=N] by proxying Subsonic getCoverArt.
/// Future offline-cache interception point.
pub fn handle<R: tauri::Runtime>(
    ctx: UriSchemeContext<'_, R>,
    request: Request<Vec<u8>>,
    responder: UriSchemeResponder,
) {
    let app = ctx.app_handle().clone();
    let uri = request.uri().clone();
    tauri::async_runtime::spawn(async move {
        let respond_err = |responder: UriSchemeResponder, status: StatusCode, msg: &str| {
            let resp = build_response(status, None, msg.as_bytes().to_vec());
            responder.respond(resp);
        };

        // path is /cover/<id> (host may hold the first segment on some platforms)
        let segments: Vec<&str> = uri
            .host()
            .into_iter()
            .chain(uri.path().split('/'))
            .filter(|s| !s.is_empty())
            .collect();
        let (kind, id) = match segments.as_slice() {
            [kind, id, ..] => (*kind, *id),
            _ => return respond_err(responder, StatusCode::BAD_REQUEST, "bad path"),
        };
        if kind != "cover" {
            return respond_err(responder, StatusCode::NOT_FOUND, "unknown resource");
        }
        let size: Option<u32> = uri
            .query()
            .and_then(|q| q.split('&').find_map(|p| p.strip_prefix("size=")))
            .and_then(|v| v.parse().ok());

        let client = match app.state::<AppState>().client() {
            Ok(c) => c,
            Err(_) => {
                return respond_err(responder, StatusCode::SERVICE_UNAVAILABLE, "not connected")
            }
        };
        let url = client.cover_art_url(id, size);
        match reqwest::get(&url).await.and_then(|r| r.error_for_status()) {
            Ok(resp) => {
                let content_type = resp
                    .headers()
                    .get("content-type")
                    .and_then(|v| v.to_str().ok())
                    .unwrap_or("image/jpeg")
                    .to_string();
                match resp.bytes().await {
                    Ok(bytes) => {
                        let http_resp =
                            build_response(StatusCode::OK, Some(&content_type), bytes.to_vec());
                        responder.respond(http_resp);
                    }
                    Err(e) => respond_err(responder, StatusCode::BAD_GATEWAY, &e.to_string()),
                }
            }
            Err(e) => respond_err(responder, StatusCode::BAD_GATEWAY, &e.to_string()),
        }
    });
}
