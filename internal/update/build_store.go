// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

//go:build store

package update

// storeBuild is true for the Microsoft Store build (`-tags store`): the in-app
// updater is compiled out and IsStoreManaged() is unconditionally true.
const storeBuild = true
