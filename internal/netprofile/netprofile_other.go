// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

//go:build !windows

package netprofile

// Only Windows classifies networks this way, and only Windows filters by that
// classification. Elsewhere there is nothing to report and nothing to warn about.
func current() Status { return Status{Category: Unknown, Supported: false} }
