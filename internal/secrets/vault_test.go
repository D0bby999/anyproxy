// SPDX-License-Identifier: AGPL-3.0-or-later

package secrets

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestVaultRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.vault")
	mem := NewMemory()
	v := NewVault(path, mem)
	if err := v.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if err := v.Set(ServiceProxy, "px-1", []byte("s3cret")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := v.Get(ServiceProxy, "px-1")
	if err != nil || string(got) != "s3cret" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	// Reopen from disk with the same key in mem.
	v2 := NewVault(path, mem)
	got2, err := v2.Get(ServiceProxy, "px-1")
	if err != nil || string(got2) != "s3cret" {
		t.Fatalf("reopen Get = %q, %v", got2, err)
	}
	if err := v2.Delete(ServiceProxy, "px-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := v2.Get(ServiceProxy, "px-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

