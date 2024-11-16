package web

import (
	"embed"
	"io/fs"
	"path"
)

//go:embed static/*
var staticFSRoot embed.FS

// myFS implements fs.FS
type staticFS struct {
	content embed.FS
}

func (c staticFS) Open(name string) (fs.File, error) {
	return c.content.Open(path.Join("static", name))
}
