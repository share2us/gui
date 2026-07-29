//go:build !store

package update

// storeBuild is false for the normal (direct-download) build: the in-app updater
// is compiled in and enabled. See build_store.go for the Store variant.
const storeBuild = false
