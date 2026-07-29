//go:build store

package update

// storeBuild is true for the Microsoft Store build (`-tags store`): the in-app
// updater is compiled out and IsStoreManaged() is unconditionally true.
const storeBuild = true
