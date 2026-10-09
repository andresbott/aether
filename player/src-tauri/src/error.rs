use aether_core::error::CoreError;
use serde::Serialize;

#[derive(Debug, Clone, Serialize)]
pub enum ErrorCode {
    NotConnected,
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

impl From<CoreError> for AppError {
    fn from(e: CoreError) -> Self {
        let code = match &e {
            CoreError::NotConnected => ErrorCode::NotConnected,
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
}
