// SPDX-License-Identifier: AGPL-3.0-or-later

// Package secrets keeps proxy passwords out of the database, logs and command
// lines. Production storage is the macOS Keychain, or Windows Credential
// Manager on Windows.
package secrets

import (
	"errors"
	"strings"
	"sync"
)

const (
	ServiceProxy    = "dev.anyprofile.anyproxyxa.proxy"
	ServiceVaultKey = "dev.anyprofile.anyproxyxa.vault-key"
)

var ErrNotFound = errors.New("secret not found")

// Store reads and writes opaque secrets addressed by service and account.
type Store interface {
	Get(service, account string) ([]byte, error)
	Set(service, account string, data []byte) error
	Delete(service, account string) error
}

// Memory is an in-process Store for tests.
type Memory struct {
	mu sync.Mutex
	m  map[string][]byte
}

func NewMemory() *Memory { return &Memory{m: map[string][]byte{}} }

func (s *Memory) Get(service, account string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[service+"\x00"+account]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), v...), nil
}

func (s *Memory) Set(service, account string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[service+"\x00"+account] = append([]byte(nil), data...)
	return nil
}

func (s *Memory) Delete(service, account string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, service+"\x00"+account)
	return nil
}

func (s *Memory) Accounts(service string) (accts []string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.m {
		if svc, acct, _ := strings.Cut(k, "\x00"); svc == service {
			accts = append(accts, acct)
		}
	}
	return accts, nil
}

// Wipe zeroes a secret buffer once it has been handed off.
func Wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

