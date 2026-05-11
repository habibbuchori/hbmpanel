package web

import (
	"embed"
	"io/fs"
)

// Folder dist/ diisi oleh `pnpm build` di frontend/, lalu hasil out/ dicopy ke
// backend/internal/web/dist sebelum `go build`. File .gitkeep menjaga folder
// tetap eksis saat dev sehingga embed tidak gagal kompilasi.
//
//go:embed all:dist
var content embed.FS

// FS mengembalikan sub-fs ke dist/, atau nil bila kosong.
func FS() fs.FS {
	sub, err := fs.Sub(content, "dist")
	if err != nil {
		return nil
	}
	return sub
}
