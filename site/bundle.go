//go:build sitebundle

package site

import (
	"embed"
	"io/fs"
)

//go:embed all:www
var www embed.FS

func init() { files, _ = fs.Sub(www, "www") }
