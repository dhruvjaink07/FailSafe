package main

import (
	"os"
	"strings"
	"testing"
)

func TestResolveDBConnStringUsesDBURLWhenSet(t *testing.T) {
	t.Setenv("DB_URL", "postgres://u:p@host:5432/db?sslmode=require")
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_NAME", "")

	got, err := resolveDBConnString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "postgres://u:p@host:5432/db?sslmode=require" {
		t.Fatalf("unexpected dsn: %s", got)
	}
}

func TestResolveDBConnStringBuildsLocalDisableSSL(t *testing.T) {
	t.Setenv("DB_URL", "")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "failsafe")
	t.Setenv("DB_PASSWORD", "failsafe")
	t.Setenv("DB_NAME", "failsafe")
	t.Setenv("DB_SSLMODE", "")

	got, err := resolveDBConnString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Fatalf("expected non-empty dsn")
	}
	if !strings.Contains(got, "sslmode=disable") {
		t.Fatalf("expected sslmode=disable, got: %s", got)
	}
}

func TestResolveDBConnStringBuildsRemoteRequireSSL(t *testing.T) {
	t.Setenv("DB_URL", "")
	t.Setenv("DB_HOST", "dpg-abcd1234.oregon-postgres.render.com")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "failsafe_dev")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "failsafe")
	t.Setenv("DB_SSLMODE", "")

	got, err := resolveDBConnString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "sslmode=require") {
		t.Fatalf("expected sslmode=require, got: %s", got)
	}
}

func TestResolveDBConnStringMissingRequired(t *testing.T) {
	t.Setenv("DB_URL", "")
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_NAME", "")

	_, err := resolveDBConnString()
	if err == nil {
		t.Fatalf("expected error for missing db config")
	}
}

func TestEnsureDefaultConfigParams(t *testing.T) {
	for _, k := range []string{"CONFIG_PARAM_1", "CONFIG_PARAM_2", "CONFIG_PARAM_3", "CONFIG_PARAM_4", "CONFIG_PARAM_5"} {
		_ = os.Unsetenv(k)
	}

	ensureDefaultConfigParams()

	for _, k := range []string{"CONFIG_PARAM_1", "CONFIG_PARAM_2", "CONFIG_PARAM_3", "CONFIG_PARAM_4", "CONFIG_PARAM_5"} {
		if os.Getenv(k) == "" {
			t.Fatalf("expected %s to be set", k)
		}
	}
}
