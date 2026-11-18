// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !darwin && !windows

package update

import "errors"

// Apply is only supported on macOS and Windows desktop builds.
func Apply(pkg string, pid int) error {
	return errors.New("in-app updates are only supported on macOS and Windows")
}

// CleanStale is a no-op on unsupported platforms.
func CleanStale() {}
