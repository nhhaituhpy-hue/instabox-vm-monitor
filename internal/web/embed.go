package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var rawStatic embed.FS

func StaticFS() fs.FS {
	sub, err := fs.Sub(rawStatic, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
