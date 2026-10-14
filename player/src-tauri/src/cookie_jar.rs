//! The HTTP plugin's cookie jar (`app_cache_dir()/.cookies`) holds the session
//! cookie, which reaches /api/v0 and can mint API keys — more than the key
//! `credentials.rs` keeps owner-only. The plugin creates the file with the
//! default mode and rewrites it in place (`tokio::fs::write`), so tightening it
//! once at startup holds: the plugin's setup has created it by then, and no
//! response has saved a cookie in it yet.
use std::io;
use std::path::Path;

/// The jar's file name in the app cache directory, as tauri-plugin-http names it.
#[cfg_attr(not(unix), allow(dead_code))]
const COOKIE_JAR_FILE: &str = ".cookies";

/// Makes the cookie jar in `cache_dir` readable by its owner only. A jar that
/// does not exist is left alone.
#[cfg(unix)]
pub fn restrict_to_owner(cache_dir: &Path) -> io::Result<()> {
    use std::fs;
    use std::os::unix::fs::PermissionsExt;
    let jar = cache_dir.join(COOKIE_JAR_FILE);
    match fs::set_permissions(jar, fs::Permissions::from_mode(0o600)) {
        Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(()),
        result => result,
    }
}

/// Windows: the per-user AppData directory is the protection, as for
/// `credentials.json`.
#[cfg(not(unix))]
pub fn restrict_to_owner(_cache_dir: &Path) -> io::Result<()> {
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use std::path::PathBuf;

    /// A directory under the system temp dir that does not exist yet, unique
    /// to the calling test.
    fn fresh_dir(test: &str) -> PathBuf {
        let dir =
            std::env::temp_dir().join(format!("aether-cookie-jar-{}-{test}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        dir
    }

    // The plugin creates the jar with the default mode, usually world-readable,
    // and the session cookie in it reaches /api/v0 and can mint API keys.
    #[cfg(unix)]
    #[test]
    fn the_jar_becomes_readable_by_its_owner_only() {
        use std::os::unix::fs::PermissionsExt;
        let dir = fresh_dir("tighten");
        fs::create_dir_all(&dir).unwrap();
        let jar = dir.join(COOKIE_JAR_FILE);
        fs::write(&jar, r#"[{"name":"session"}]"#).unwrap();
        fs::set_permissions(&jar, fs::Permissions::from_mode(0o644)).unwrap();

        restrict_to_owner(&dir).unwrap();

        assert_eq!(
            fs::metadata(&jar).unwrap().permissions().mode() & 0o777,
            0o600
        );
        // tightening must not cost the session
        assert_eq!(fs::read_to_string(&jar).unwrap(), r#"[{"name":"session"}]"#);
        fs::remove_dir_all(&dir).unwrap();
    }

    // The jar is the plugin's: when there is none, there is nothing to tighten,
    // and creating one is not this code's business.
    #[test]
    fn a_missing_jar_is_left_alone() {
        let dir = fresh_dir("missing");

        restrict_to_owner(&dir).unwrap();

        assert!(!dir.join(COOKIE_JAR_FILE).exists(), "a jar was created");
    }
}
