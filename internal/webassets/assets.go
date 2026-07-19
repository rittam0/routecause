package webassets

import (
	"embed"
	"io/fs"
)

//go:embed dist/* dist/assets/*
var embedded embed.FS

func Assets() (fs.FS, error) { return fs.Sub(embedded, "dist") }
