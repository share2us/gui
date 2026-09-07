// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

//go:build !windows

package netprofile

import "errors"

// OpenSettings has nothing to open: no other platform filters by a network
// classification like this.
func OpenSettings() error { return errors.New("only Windows classifies networks this way") }
