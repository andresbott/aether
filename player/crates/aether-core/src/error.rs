use thiserror::Error;

#[derive(Debug, Error)]
pub enum CoreError {
    #[error("not connected to a server")]
    NotConnected,
    #[error("server unreachable: {0}")]
    ServerUnreachable(String),
    #[error("authentication failed")]
    AuthFailed,
    #[error("server error {code}: {message}")]
    Api { code: u32, message: String },
    #[error("invalid server response: {0}")]
    InvalidResponse(String),
    #[error("audio decode error: {0}")]
    Decode(String),
    #[error("engine error: {0}")]
    Engine(String),
}
