package config

import (
	"net/url"
	"testing"
)

func TestBuildDatabaseURL_EscapesPassword(t *testing.T) {
	raw := BuildDatabaseURL("db.local", "5432", "user", "p@ss!w$rd", "app", "require")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url should parse: %v", err)
	}
	pw, _ := u.User.Password()
	if pw != "p@ss!w$rd" || u.Host != "db.local:5432" || u.Path != "/app" || u.Query().Get("sslmode") != "require" {
		t.Fatalf("unexpected url: %s", raw)
	}
}

func TestSSLMode(t *testing.T) {
	cases := map[string]string{"": "disable", "false": "disable", "true": "require", "require": "require", "verify-full": "verify-full"}
	for in, want := range cases {
		if got := SSLMode(in); got != want {
			t.Errorf("SSLMode(%q) = %q, want %q", in, got, want)
		}
	}
}
