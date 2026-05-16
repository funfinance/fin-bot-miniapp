package webfs

import (
	"embed"
	"io/fs"
)

//go:embed web
var files embed.FS

// FS returns the embedded web filesystem rooted at the "web" subdirectory.
func FS() fs.FS {
	sub, _ := fs.Sub(files, "web")
	return sub
}
