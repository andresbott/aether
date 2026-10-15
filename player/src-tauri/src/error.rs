use aether_core::error::CoreError;
use serde::Serialize;

#[derive(Debug, Clone, Serialize)]
pub enum ErrorCode {
    NotConnected,
    InvalidServerUrl,
    ServerUnreachable,
    AuthFailed,
    ServerError,
    DecodeError,
    Internal,
}

#[derive(Debug, Clone, Serialize)]
pub struct AppError {
    pub code: ErrorCode,
    pub message: String,
}

impl AppError {
    /// A failure that is neither the server's nor the caller's, such as the
    /// settings file not being writable.
    pub fn internal(message: impl Into<String>) -> Self {
        AppError {
            code: ErrorCode::Internal,
            message: message.into(),
        }
    }
}

impl From<CoreError> for AppError {
    fn from(e: CoreError) -> Self {
        let code = match &e {
            CoreError::NotConnected => ErrorCode::NotConnected,
            CoreError::InvalidServerUrl(_) => ErrorCode::InvalidServerUrl,
            CoreError::ServerUnreachable(_) => ErrorCode::ServerUnreachable,
            CoreError::AuthFailed => ErrorCode::AuthFailed,
            CoreError::Api { .. } | CoreError::InvalidResponse(_) => ErrorCode::ServerError,
            CoreError::Decode(_) => ErrorCode::DecodeError,
            CoreError::Engine(_) => ErrorCode::Internal,
        };
        AppError {
            code,
            message: e.to_string(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use aether_core::error::CoreError;

    #[test]
    fn core_errors_map_to_stable_codes() {
        let e: AppError = CoreError::AuthFailed.into();
        assert!(matches!(e.code, ErrorCode::AuthFailed));

        let e: AppError = CoreError::NotConnected.into();
        assert!(matches!(e.code, ErrorCode::NotConnected));

        let e: AppError = CoreError::InvalidServerUrl("missing scheme".into()).into();
        assert!(matches!(e.code, ErrorCode::InvalidServerUrl));

        let e: AppError = CoreError::ServerUnreachable("x".into()).into();
        assert!(matches!(e.code, ErrorCode::ServerUnreachable));

        let e: AppError = CoreError::Decode("bad mp3".into()).into();
        assert!(matches!(e.code, ErrorCode::DecodeError));
    }

    #[test]
    fn app_error_serializes_to_code_and_message() {
        let e: AppError = CoreError::AuthFailed.into();
        let v = serde_json::to_value(&e).unwrap();
        assert_eq!(v["code"], "AuthFailed");
        assert!(v["message"].is_string());
    }

    #[test]
    fn invalid_server_url_serializes_to_its_own_code() {
        // the SPA branches on this string
        let e: AppError = CoreError::InvalidServerUrl("missing scheme".into()).into();
        let v = serde_json::to_value(&e).unwrap();
        assert_eq!(v["code"], "InvalidServerUrl");
    }

    #[test]
    fn internal_errors_carry_the_given_message() {
        let v = serde_json::to_value(AppError::internal("disk full")).unwrap();
        assert_eq!(v["code"], "Internal");
        assert_eq!(v["message"], "disk full");
    }
}
