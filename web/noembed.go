//go:build !embed_frontend

package webassets

import "io/fs"

func FS() fs.FS {
	return nil
}
