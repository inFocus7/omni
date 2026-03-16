package store

import "context"

// RemoteStore extends Store with synchronisation and health-check operations.
// It is an extension point for future external-app integration; no implementation
// is provided yet.
type RemoteStore interface {
	Store

	// Sync pushes/pulls the latest state from the remote peer.
	Sync(ctx context.Context) error

	// Healthy reports whether the remote peer is reachable.
	Healthy(ctx context.Context) bool
}
