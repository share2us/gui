// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

//go:build windows

package netprofile

import "os/exec"

// OpenSettings shows the Windows network settings page, where the Public/Private
// classification is changed.
//
// The app does not change it: that needs administrator rights, and deciding a
// network is trusted is a judgement about the user's surroundings, not something
// a file-sharing app should do on one click.
//
// explorer.exe is used rather than the generic URL opener, which refuses
// non-web schemes on purpose, and rather than a shell, which would flash a
// console. The target is a constant, so nothing user-supplied reaches it.
func OpenSettings() error {
	return exec.Command("explorer.exe", "ms-settings:network").Start()
}
