// Package migrations embeds the core SQL migrations into the binary.
//
// The runner used to open "file://migrations/versioned", a path relative to
// the working directory: a server started from anywhere but the repository
// root (a systemd unit, a bare binary copied to /usr/local/bin) found no
// migrations and either failed or — worse, before start-up became fail-fast —
// carried on against an unmigrated schema. Embedding ties the schema to the
// binary that expects it. The SQL stays on disk as ordinary files, so
// scripts/migrate.sh (which drives the migrate CLI against the directory) and
// code review work as before.
package migrations

import (
	"embed"
	"io/fs"
)

//go:embed versioned/*.sql
var versioned embed.FS

// Core returns the core migrations: NNNNNN_name.up.sql / .down.sql files at
// the root of the returned file system, in the layout golang-migrate's iofs
// source expects.
func Core() fs.FS {
	sub, err := fs.Sub(versioned, "versioned")
	if err != nil {
		// fs.Sub only fails on an invalid path, and the path is a constant.
		panic("migrations: " + err.Error())
	}
	return sub
}
