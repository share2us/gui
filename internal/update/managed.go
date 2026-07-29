package update

// IsStoreManaged reports whether in-app self-update must be disabled because this
// copy is distributed through the Microsoft Store. It is true when EITHER the
// binary was built for the Store (the `store` build tag — compile-time, belt) OR
// the process is running with MSIX package identity (runtime detection —
// suspenders). Either way satisfies Store policy 10.2.5, which forbids a Store
// app from updating itself outside the Store.
//
// The two guards are complementary: the build tag removes the updater code from
// the Store binary, while the runtime check protects any binary that ends up
// packaged, so a Store install can never self-update even by accident.
func IsStoreManaged() bool {
	return storeBuild || runningPackaged()
}
