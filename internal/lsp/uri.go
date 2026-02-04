package lsp

import (
	"net/url"
	"path/filepath"
	"strings"
)

func UriToPath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return ""
	}
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	pth, err := url.PathUnescape(u.Path)
	if err != nil {
		return ""
	}
	return filepath.FromSlash(pth)
}

func PathToURI(absPath string) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(absPath)}
	return u.String()
}
