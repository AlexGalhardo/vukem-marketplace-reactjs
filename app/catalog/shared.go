package catalog

import (
	"crypto/sha256"
	"embed"
	"encoding/base32"
	"path"
	"regexp"
	"strings"
)

// sharedFS holds one small WebP per category (see
// internal/tools/genplaceholders). Demo products point at these through
// SharedKey, so their images live in the binary: no disk, no network.
//
//go:embed assets/shared/*.webp
var sharedFS embed.FS

// sharedVariantRe strips the optional width suffix of a stored key.
var sharedVariantRe = regexp.MustCompile(`-w(320|640|1280)(\.webp)$`)

// SharedKey is the stored key of the shared image of a category: a fixed
// name in the catalog's object format under SharedDir.
func SharedKey(slug string) string {
	sum := sha256.Sum256([]byte("vukem-demo-placeholder:" + slug))
	name := strings.ToLower(base32.StdEncoding.EncodeToString(sum[:]))[:26]

	return SharedDir + "/" + name + ".webp"
}

// SharedImage returns the embedded image behind a shared-dir object path
// (any width variant of a SharedKey).
func SharedImage(p string) ([]byte, bool) {
	if !strings.HasPrefix(p, SharedDir+"/") {
		return nil, false
	}

	p = sharedVariantRe.ReplaceAllString(p, "$2")

	entries, err := sharedFS.ReadDir("assets/shared")
	if err != nil {
		return nil, false
	}

	for _, e := range entries {
		slug := strings.TrimSuffix(e.Name(), path.Ext(e.Name()))
		if SharedKey(slug) != p {
			continue
		}

		b, err := sharedFS.ReadFile("assets/shared/" + e.Name())

		return b, err == nil
	}

	return nil, false
}

// SharedImageSize is the side in pixels of the embedded images.
const SharedImageSize = 640
