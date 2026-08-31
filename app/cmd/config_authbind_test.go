package cmd

import (
	"strings"
	"testing"
)

// Under auth "none" the server is unauthenticated, so it may not listen on
// every interface: an unset BindIp resolves to loopback, an explicit wildcard
// is refused, and a specific address (e.g. a LAN interface an auth proxy on
// another host reaches) is allowed. The authenticated modes are unrestricted.

func TestAuthNoneUnsetBindIpResolvesToLoopback(t *testing.T) {
	cfg, err := getAppCfg(writeCfg(t, "Auth:\n  Method: \"none\"\n"), true)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if cfg.Server.BindIp != "127.0.0.1" {
		t.Fatalf("expected BindIp resolved to 127.0.0.1, got %q", cfg.Server.BindIp)
	}
}

func TestAuthNoneRejectsWildcardBind(t *testing.T) {
	for _, bind := range []string{"0.0.0.0", "::", "[::]"} {
		t.Run(bind, func(t *testing.T) {
			body := "Auth:\n  Method: \"none\"\nServer:\n  BindIp: \"" + bind + "\"\n"
			_, err := getAppCfg(writeCfg(t, body), true)
			if err == nil {
				t.Fatalf("expected error for wildcard bind %q under auth none, got nil", bind)
			}
			if !strings.Contains(err.Error(), "BindIp") {
				t.Fatalf("error should mention BindIp, got: %v", err)
			}
		})
	}
}

func TestAuthNoneAllowsSpecificBind(t *testing.T) {
	body := "Auth:\n  Method: \"none\"\nServer:\n  BindIp: \"192.168.1.50\"\n"
	cfg, err := getAppCfg(writeCfg(t, body), true)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if cfg.Server.BindIp != "192.168.1.50" {
		t.Fatalf("expected BindIp 192.168.1.50, got %q", cfg.Server.BindIp)
	}
}

func TestNativeAllowsWildcardBind(t *testing.T) {
	body := "Auth:\n  Method: \"native\"\nServer:\n  BindIp: \"0.0.0.0\"\n"
	cfg, err := getAppCfg(writeCfg(t, body), true)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if cfg.Server.BindIp != "0.0.0.0" {
		t.Fatalf("expected BindIp 0.0.0.0 unchanged under native, got %q", cfg.Server.BindIp)
	}
}

func TestNativeUnsetBindIpStaysAllInterfaces(t *testing.T) {
	cfg, err := getAppCfg(writeCfg(t, "Auth:\n  Method: \"native\"\n"), true)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if cfg.Server.BindIp != "" {
		t.Fatalf("expected BindIp unset under native, got %q", cfg.Server.BindIp)
	}
}

func TestProxyHeaderAllowsWildcardBind(t *testing.T) {
	body := "Auth:\n  Method: \"proxy-header\"\nServer:\n  BindIp: \"0.0.0.0\"\n"
	cfg, err := getAppCfg(writeCfg(t, body), true)
	if err != nil {
		t.Fatalf("getAppCfg: %v", err)
	}
	if cfg.Server.BindIp != "0.0.0.0" {
		t.Fatalf("expected BindIp 0.0.0.0 unchanged under proxy-header, got %q", cfg.Server.BindIp)
	}
}
