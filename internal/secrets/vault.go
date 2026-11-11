// SPDX-License-Identifier: AGPL-3.0-or-later

package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
)

// Vault keeps every secret in one encrypted file; only the file's key is in
// the system store (Keychain, Credential Manager).
type Vault struct {
	store Store
	path  string

	mu     sync.Mutex
	aead   cipher.AEAD
	items  map[string][]byte
	gone   map[string]bool
	loaded bool
}

var ErrVaultKeyLost = errors.New("the key of the secrets file is missing from the system keychain")

const vaultAAD = "anyproxyxa-vault-v1"

func NewVault(path string, store Store) *Vault { return &Vault{store: store, path: path} }

type vaultFile struct {
	Items map[string][]byte `json:"items"`
	Gone  []string          `json:"gone,omitempty"`
}

func (v *Vault) Unlock() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.unlock()
}

func (v *Vault) unlock() error {
	if v.loaded {
		return nil
	}
	sealed, err := os.ReadFile(v.path)
	exists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	key, err := v.store.Get(ServiceVaultKey, "app")
	switch {
	case errors.Is(err, ErrNotFound) && exists:
		return ErrVaultKeyLost
	case errors.Is(err, ErrNotFound):
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return err
		}
		if err := v.store.Set(ServiceVaultKey, "app", key); err != nil {
			return err
		}
	case err != nil:
		return err
	}
	defer Wipe(key)
	if len(key) != 32 {
		return fmt.Errorf("vault key has %d bytes, want 32", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	f := vaultFile{Items: map[string][]byte{}}
	if exists {
		ns := aead.NonceSize()
		if len(sealed) < ns {
			return fmt.Errorf("read %s: too short", v.path)
		}
		plain, err := aead.Open(nil, sealed[:ns], sealed[ns:], []byte(vaultAAD))
		if err != nil {
			return fmt.Errorf("read %s: %w", v.path, err)
		}
		err = json.Unmarshal(plain, &f)
		Wipe(plain)
		if err != nil {
			return fmt.Errorf("read %s: %w", v.path, err)
		}
	}
	v.aead, v.items, v.gone, v.loaded = aead, f.Items, map[string]bool{}, true
	if v.items == nil {
		v.items = map[string][]byte{}
	}
	for _, k := range f.Gone {
		v.gone[k] = true
	}
	return nil
}

func (v *Vault) save() error {
	f := vaultFile{Items: v.items}
	for k := range v.gone {
		f.Gone = append(f.Gone, k)
	}
	plain, err := json.Marshal(f)
	if err != nil {
		return err
	}
	defer Wipe(plain)
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	tmp := v.path + ".tmp"
	if err := os.WriteFile(tmp, v.aead.Seal(nonce, nonce, plain, []byte(vaultAAD)), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, v.path)
}

func vaultKey(service, account string) string { return service + "\x00" + account }

func (v *Vault) Get(service, account string) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.unlock(); err != nil {
		return nil, err
	}
	k := vaultKey(service, account)
	if s, ok := v.items[k]; ok {
		return append([]byte(nil), s...), nil
	}
	if v.gone[k] {
		return nil, ErrNotFound
	}
	return nil, ErrNotFound
}

func (v *Vault) Set(service, account string, data []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.unlock(); err != nil {
		return err
	}
	k := vaultKey(service, account)
	old, had := v.items[k]
	wasGone := v.gone[k]
	v.items[k] = append([]byte(nil), data...)
	delete(v.gone, k)
	if err := v.save(); err != nil {
		if had {
			v.items[k] = old
		} else {
			delete(v.items, k)
		}
		if wasGone {
			v.gone[k] = true
		}
		return err
	}
	return nil
}

func (v *Vault) Delete(service, account string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.unlock(); err != nil {
		return err
	}
	k := vaultKey(service, account)
	old, had := v.items[k]
	wasGone := v.gone[k]
	delete(v.items, k)
	v.gone[k] = true
	if err := v.save(); err != nil {
		if had {
			v.items[k] = old
		}
		if !wasGone {
			delete(v.gone, k)
		}
		return err
	}
	return nil
}
