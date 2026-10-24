# Demo data and images

## What `db:seed` creates

- Reference data: the 12 categories (English and Portuguese names, because the UI is bilingual).
- Demo accounts (development only): `buyer@`, `seller@` and `admin@example.com`.
- The demo shop with 12 products, and the demo catalog: 100 shops, 2000 products, 300 buyers, about 1500 orders, 1200 reviews and 1200 questions.

All text is English: product titles, descriptions, reviews, questions, answers, chat scripts and buyer names. Buyer delivery addresses use real Brazilian cities and CEP ranges because shipping and freight are computed from them.

The generator is deterministic (`DemoCatalogSeed`), so the same seed always gives the same data. `db:seed` is idempotent and refuses production; the catalog alone is opt-in there with `demo:catalog --allow-production`.

## Images

Demo products point at one small WebP per category, embedded in the Go binary:

```text
app/catalog/assets/shared/<category>.webp     640x640, 2 to 8 KB each
catalog.SharedKey(slug)                      stored key under products/0/
catalog.SharedImage(path)                    embedded bytes for any width variant
```

`GET /media/products/0/...` serves them straight from memory, so they never depend on the disk, the bucket or an external URL, and a redeploy cannot break them.

To change the look, edit `internal/tools/genplaceholders/main.go` (a gradient and one white glyph per category) and run:

```sh
go run internal/tools/genplaceholders/main.go
```

It needs `ffmpeg` with libwebp and rewrites the files in `app/catalog/assets/shared`; commit the result.

Real seller uploads still go to S3 and get their 320/640/1280 variants from a queue job.
