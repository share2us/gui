// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

//go:build !windows

package update

// runningPackaged is always false off Windows: MSIX package identity is a Windows
// concept and the Microsoft Store distribution only applies to the Windows build.
func runningPackaged() bool { return false }
