package main

import "os"
import "testing"

func TestIsTrustedModule_Default(t *testing.T) {
	os.Unsetenv("FIELDCTL_TRUSTED_MODULE_PREFIXES")
	if !isTrustedModule(DefaultTrustedModulePrefix + "foo") {
		t.Fatalf("default prefix not trusted when env unset")
	}
	if isTrustedModule("example.com/foo") {
		t.Fatalf("untrusted module allowed")
	}
}

func TestIsTrustedModule_Empty(t *testing.T) {
	os.Setenv("FIELDCTL_TRUSTED_MODULE_PREFIXES", "")
	defer os.Unsetenv("FIELDCTL_TRUSTED_MODULE_PREFIXES")
	if !isTrustedModule(DefaultTrustedModulePrefix + "bar") {
		t.Fatalf("default prefix not trusted when env empty")
	}
}

func TestIsTrustedModule_Single(t *testing.T) {
	os.Setenv("FIELDCTL_TRUSTED_MODULE_PREFIXES", "example.com/mod/")
	defer os.Unsetenv("FIELDCTL_TRUSTED_MODULE_PREFIXES")
	if !isTrustedModule("example.com/mod/x") {
		t.Fatalf("custom prefix not allowed")
	}
	if isTrustedModule(DefaultTrustedModulePrefix + "x") {
		t.Fatalf("default prefix allowed when env specifies custom prefix")
	}
}

func TestIsTrustedModule_CommaSeparated(t *testing.T) {
	os.Setenv("FIELDCTL_TRUSTED_MODULE_PREFIXES", "example.com/a/, example.org/b/")
	defer os.Unsetenv("FIELDCTL_TRUSTED_MODULE_PREFIXES")
	if !isTrustedModule("example.com/a/foo") || !isTrustedModule("example.org/b/foo") {
		t.Fatalf("comma separated prefixes not handled")
	}
	if isTrustedModule(DefaultTrustedModulePrefix + "x") {
		t.Fatalf("unlisted prefix allowed")
	}
}
