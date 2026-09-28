package cmd

import "testing"

// LoginThrottle.Enabled defaults to on: only an explicit false turns the
// brute-force guard off. An omitted, blank or null key must keep the default,
// so the check goes through the real loader, not a hand-built AuthCfg.
func TestLoginThrottleEnabledFromConfig(t *testing.T) {
	const native = "Auth:\n  Method: \"native\"\n  AdminBootstrap:\n    User: \"admin\"\n    Pw: \"admin\"\n"
	for name, tc := range map[string]struct {
		body string
		want bool
	}{
		"omitted":        {body: native, want: true},
		"blank":          {body: native + "  LoginThrottle:\n    Enabled:\n", want: true},
		"null":           {body: native + "  LoginThrottle:\n    Enabled: ~\n", want: true},
		"explicit true":  {body: native + "  LoginThrottle:\n    Enabled: true\n", want: true},
		"explicit false": {body: native + "  LoginThrottle:\n    Enabled: false\n", want: false},
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := getAppCfg(writeCfg(t, tc.body), true)
			if err != nil {
				t.Fatalf("getAppCfg: %v", err)
			}
			if got := cfg.Auth.LoginThrottle.enabled(); got != tc.want {
				t.Fatalf("LoginThrottle enabled = %v, want %v", got, tc.want)
			}
		})
	}
}
