package catalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png" // PNG decoder for uploads
	"io"
	"regexp"
	"strconv"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/storage"
	"github.com/AlexGalhardo/vukem/upload"
)

// Image limits. MaxImagePixels bounds the decoded size (16 MP is 64 MB as
// RGBA), well below upload.DefaultMaxPixels, because every image is decoded
// to be resized.
const (
	MaxImageBytes  = 5 << 20
	MaxImagePixels = 16_000_000
	jpegQuality    = 82
)

// Widths are the stored variants of every image, largest first; images
// narrower than a width are stored at their own width (never upscaled).
var Widths = [...]int{1280, 640, 320}

// ImageRules is what the image upload routes accept: JPEG and PNG only
// (the standard library cannot decode WebP, and every file is re-encoded),
// sniffed from the content, extension checked, header pixel cap checked by
// vukem before anything is decoded.
func ImageRules(maxFiles int) upload.Rules {
	return upload.Rules{
		MaxFileSize: MaxImageBytes,
		MaxFiles:    max(maxFiles, 1),
		MaxPixels:   MaxImagePixels,
		Types:       []string{"image/jpeg", "image/png"},
		Extensions:  []string{".jpg", ".jpeg", ".png"},
		Dir:         "incoming",
	}
}

// keyRe is the only shape of a stored image key: <kind>/<owner id>/<128-bit
// random hex>. Keys never contain client input.
var keyRe = regexp.MustCompile(`^(products|shops)/\d{1,19}/[0-9a-f]{32}$`)

// MediaURL is the public URL of an image variant (served by the media
// route, which validates the key; the bucket itself stays private).
func MediaURL(key string, width int) string {
	return "/media/" + key + "/w" + strconv.Itoa(width) + ".jpg"
}

// MediaObject validates the parts of a media URL and returns the object
// path on the disk. Anything but a well-formed key and a known variant
// name is rejected, so the route can never read arbitrary objects.
func MediaObject(kind, id, token, file string) (string, bool) {
	key := kind + "/" + id + "/" + token
	if !keyRe.MatchString(key) {
		return "", false
	}

	for _, w := range Widths {
		if file == "w"+strconv.Itoa(w)+".jpg" {
			return key + "/" + file, true
		}
	}

	return "", false
}

func errUnreadableImage() error {
	return invalid("images", "The image could not be read. Upload a JPEG or PNG file.")
}

// encodedImage holds the JPEG variants of one image.
type encodedImage struct {
	width, height int
	variants      map[int][]byte
}

// processImage decodes an image (after re-checking its header dimensions:
// defense in depth over upload.Rules.MaxPixels) and re-encodes it as JPEG
// at every width. Re-encoding drops anything appended to or embedded in
// the file (polyglots, metadata such as GPS EXIF).
func processImage(r io.Reader) (encodedImage, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxImageBytes+1))
	if err != nil {
		return encodedImage{}, fmt.Errorf("read image: %w", err)
	}

	if len(data) > MaxImageBytes {
		return encodedImage{}, upload.ErrFileTooLarge
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxImagePixels {
		return encodedImage{}, errUnreadableImage()
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return encodedImage{}, errUnreadableImage()
	}

	// Flatten onto white: JPEG has no alpha, and transparent PNG pixels
	// would otherwise turn black.
	b := src.Bounds()
	cur := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(cur, cur.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(cur, cur.Bounds(), src, b.Min, draw.Over)

	out := encodedImage{width: b.Dx(), height: b.Dy(), variants: map[int][]byte{}}

	for _, w := range Widths {
		cur = downscale(cur, w) // each width from the previous, cheaper and as sharp

		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, cur, &jpeg.Options{Quality: jpegQuality}); err != nil {
			return encodedImage{}, fmt.Errorf("encode image: %w", err)
		}

		out.variants[w] = buf.Bytes()
	}

	return out, nil
}

// downscale shrinks src (origin 0,0) to width w with a box filter: each
// destination pixel is the average of the source pixels it covers. Images
// at most w wide are returned as is.
func downscale(src *image.RGBA, w int) *image.RGBA {
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	if sw <= w {
		return src
	}

	h := max(1, sh*w/sw)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	for dy := range h {
		y0 := dy * sh / h
		y1 := max((dy+1)*sh/h, y0+1)

		for dx := range w {
			x0 := dx * sw / w
			x1 := max((dx+1)*sw/w, x0+1)

			var sum [4]int

			for y := y0; y < y1; y++ {
				off := y*src.Stride + x0*4
				for range x1 - x0 {
					sum[0] += int(src.Pix[off])
					sum[1] += int(src.Pix[off+1])
					sum[2] += int(src.Pix[off+2])
					sum[3] += int(src.Pix[off+3])
					off += 4
				}
			}

			n := (y1 - y0) * (x1 - x0)
			i := dst.PixOffset(dx, dy)

			for k, v := range sum {
				dst.Pix[i+k] = uint8(v / n) //nolint:gosec // G115: an average of uint8 values fits in uint8
			}
		}
	}

	return dst
}

// storeVariants writes every variant under a fresh key and returns it;
// on failure nothing is left behind.
func (s *Service) storeVariants(ctx context.Context, kind string, ownerID int64, img encodedImage) (string, error) {
	key := kind + "/" + strconv.FormatInt(ownerID, 10) + "/" + randomSuffix(16)

	for _, w := range Widths {
		if err := storage.PutBytes(ctx, s.disk, key+"/w"+strconv.Itoa(w)+".jpg", img.variants[w]); err != nil {
			s.deleteVariants(ctx, key)

			return "", fmt.Errorf("store image %s: %w", key, err)
		}
	}

	return key, nil
}

// deleteVariants removes an image's objects; failures only leave orphans
// behind, so they are logged, not returned.
func (s *Service) deleteVariants(ctx context.Context, key string) {
	for _, w := range Widths {
		p := key + "/w" + strconv.Itoa(w) + ".jpg"
		if err := s.disk.Delete(ctx, p); err != nil && !errors.Is(err, storage.ErrNotFound) {
			s.log.WarnContext(ctx, "catalog: delete image variant", "path", p, "error", err)
		}
	}
}

// ImageSlots returns how many more images the acting user's product can
// take (ErrNotFound when the product is not theirs).
func (s *Service) ImageSlots(ctx context.Context, userID, productID int64) (int, error) {
	if _, err := s.ownedProduct(ctx, userID, productID); err != nil {
		return 0, err
	}

	n, err := orm.QueryOf[ProductImage](s.db).Where("product_id", productID).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count images of product %d: %w", productID, err)
	}

	return max(MaxImages-int(n), 0), nil
}

// AddProductImage processes one uploaded image (already checked by
// upload.Receive with ImageRules) and appends it to the acting user's
// product.
//
// shortcut: the 8-image cap is checked before the insert, so two
// concurrent uploads can exceed it by one batch; a row lock or a count
// column would close that if it ever matters.
func (s *Service) AddProductImage(
	ctx context.Context,
	userID, productID int64,
	r io.Reader,
	alt string,
) (ImageView, error) {
	slots, err := s.ImageSlots(ctx, userID, productID)
	if err != nil {
		return ImageView{}, err
	}

	if slots == 0 {
		return ImageView{}, ErrTooManyImages
	}

	enc, err := processImage(r)
	if err != nil {
		return ImageView{}, err
	}

	key, err := s.storeVariants(ctx, "products", productID, enc)
	if err != nil {
		return ImageView{}, err
	}

	img := ProductImage{ProductID: productID, Key: key, Width: enc.width, Height: enc.height, Alt: truncate(alt, 200)}

	err = database.RunInTx(ctx, s.db, func(tx *database.Tx) error {
		var pos int
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), -1) + 1 FROM product_images WHERE product_id = `+
			tx.Dialect().Placeholder(1), productID).
			Scan(&pos); err != nil {
			return err
		}

		img.Position = pos

		return orm.Create(ctx, tx, &img)
	})
	if err != nil {
		s.deleteVariants(ctx, key)

		return ImageView{}, fmt.Errorf("save image of product %d: %w", productID, err)
	}

	return imageView(img), nil
}

// DeleteProductImage removes an image of the acting user's product.
func (s *Service) DeleteProductImage(ctx context.Context, userID, productID, imageID int64) error {
	if _, err := s.ownedProduct(ctx, userID, productID); err != nil {
		return err
	}

	img, err := orm.QueryOf[ProductImage](s.db).Where("id", imageID).Where("product_id", productID).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("load image %d: %w", imageID, err)
	}

	if _, err := orm.QueryOf[ProductImage](s.db).Where("id", img.ID).Delete(ctx); err != nil {
		return fmt.Errorf("delete image %d: %w", imageID, err)
	}

	s.deleteVariants(ctx, img.Key)

	return nil
}

// ReorderProductImages sets the order of a product's images; ids must be
// exactly the product's image ids.
func (s *Service) ReorderProductImages(ctx context.Context, userID, productID int64, ids []int64) error {
	if _, err := s.ownedProduct(ctx, userID, productID); err != nil {
		return err
	}

	current, err := orm.QueryOf[ProductImage](s.db).Where("product_id", productID).Pluck[int64](ctx, "id")
	if err != nil {
		return fmt.Errorf("load image ids of product %d: %w", productID, err)
	}

	if !sameSet(current, ids) {
		return invalid("ids", "The image list is out of date. Reload the page.")
	}

	return database.RunInTx(ctx, s.db, func(tx *database.Tx) error {
		for pos, id := range ids {
			if _, err := orm.QueryOf[ProductImage](tx).Where("id", id).Where("product_id", productID).
				Update(ctx, orm.Set{"position": pos}); err != nil {
				return fmt.Errorf("reorder image %d: %w", id, err)
			}
		}

		return nil
	})
}

// SetShopLogo replaces the acting user's shop logo.
func (s *Service) SetShopLogo(ctx context.Context, userID int64, r io.Reader) (ShopView, error) {
	shop, err := s.ownShop(ctx, userID)
	if err != nil {
		return ShopView{}, err
	}

	enc, err := processImage(r)
	if err != nil {
		return ShopView{}, err
	}

	key, err := s.storeVariants(ctx, "shops", shop.ID, enc)
	if err != nil {
		return ShopView{}, err
	}

	if _, err := orm.QueryOf[Shop](s.db).Where("id", shop.ID).Update(ctx, orm.Set{"logo_key": key}); err != nil {
		s.deleteVariants(ctx, key)

		return ShopView{}, fmt.Errorf("set logo of shop %d: %w", shop.ID, err)
	}

	if shop.LogoKey.Valid {
		s.deleteVariants(ctx, shop.LogoKey.V)
	}

	shop.LogoKey.V, shop.LogoKey.Valid = key, true

	return shopView(shop), nil
}

func sameSet(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}

	seen := make(map[int64]bool, len(a))
	for _, v := range a {
		seen[v] = true
	}

	for _, v := range b {
		if !seen[v] {
			return false
		}

		delete(seen, v)
	}

	return len(seen) == 0
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}

	return string(r[:n])
}
