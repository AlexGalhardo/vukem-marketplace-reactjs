package catalog

import (
	"bytes"
	"testing"
)

func TestSharedImage(t *testing.T) {
	t.Parallel()

	key := SharedKey("electronics")

	for _, p := range []string{key, VariantKey(key, 320), VariantKey(key, 1280)} {
		b, ok := SharedImage(p)
		if !ok || !bytes.HasPrefix(b, []byte("RIFF")) {
			t.Fatalf("SharedImage(%q) = %d bytes, %v; want an embedded WebP", p, len(b), ok)
		}

		if _, ok := MediaObject(p); !ok {
			t.Fatalf("%q is not a servable media object", p)
		}
	}

	if _, ok := SharedImage(SharedDir + "/aaaaaaaaaaaaaaaaaaaaaaaaaa.webp"); ok {
		t.Fatal("unknown shared key must not resolve")
	}
}
