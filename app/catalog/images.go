package catalog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/contrib/imaging"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/storage"
	"github.com/AlexGalhardo/vukem/upload"
	"github.com/AlexGalhardo/vukem/web"
)

// Image limits. MaxImagePixels bounds the decoded size (16 MP is 64 MB as
// RGBA, more for 16-bit PNG), well below upload.DefaultMaxPixels, because
// every image is decoded once to make its variants.
const (
	MaxImageBytes  = 5 << 20
	MaxImagePixels = 16_000_000
)

// Widths are the stored variants of every product image, largest first;
// images narrower than a width keep their own width (never upscaled).
var Widths = [...]int{1280, 640, 320}

// logoWidth is the only variant of a shop logo.
const logoWidth = 320

// imageRules is what the image uploads accept: JPEG, PNG, GIF and WebP,
// sniffed from the content, extension checked, header pixel cap checked
// by vukem before anything is decoded. Files land under dir with random
// names.
func imageRules(dir string, maxFiles int) upload.Rules {
	r := upload.Images(MaxImageBytes)
	r.MaxFiles = max(maxFiles, 1)
	r.MaxPixels = MaxImagePixels
	r.Dir = dir

	return r
}

// Stored object names: <products|shops>/<owner id>/<upload random
// name>[-w<width>].<ext>. Keys never contain client input.
var objectRe = regexp.MustCompile(`^(products|shops)/\d{1,19}/[a-z2-7]{26}(-w(320|640|1280))?\.(jpg|png|gif|webp)$`)

// mediaTypes maps the extensions objectRe allows to their content type.
var mediaTypes = map[string]string{
	".jpg": "image/jpeg", ".png": "image/png", ".gif": "image/gif", ".webp": "image/webp",
}

// MediaObject validates the object path of a media URL (the part after
// /media/) and returns its content type. Anything but a stored image name
// is rejected, so the route can never read arbitrary objects.
func MediaObject(p string) (contentType string, ok bool) {
	if !objectRe.MatchString(p) {
		return "", false
	}

	return mediaTypes[path.Ext(p)], true
}

// variantPath is the object holding the width variant of a processed
// image whose key is <base>.<ext> (ext is the variants' format).
func variantPath(key string, width int) string {
	ext := path.Ext(key)

	return strings.TrimSuffix(key, ext) + "-w" + strconv.Itoa(width) + ext
}

// imageURL is the URL of an image at width: the variant once the
// thumbnail job ran (processed), the uploaded original until then.
func imageURL(key string, processed bool, width int) string {
	if !processed {
		return "/media/" + key
	}

	return "/media/" + variantPath(key, width)
}

// VariantKey is the stored object of the width variant of a processed
// image key.
func VariantKey(key string, width int) string { return variantPath(key, width) }

// SharedDir holds placeholder images that many demo products share
// (database/seeders); no product has id 0. Deleting one product's image
// must not remove them, so deleteObject keeps everything under it.
const SharedDir = "products/0"

// MediaURL is the public URL of the width variant of a processed product
// image key (served by the media route; the bucket itself stays private).
func MediaURL(key string, width int) string { return imageURL(key, true, width) }

// ThumbnailJob makes the variants of an uploaded image off the request
// path. Target "product" carries a product_images id, "shop" a shop id;
// Src is the uploaded original, so a job for a replaced logo or a deleted
// image does nothing.
type ThumbnailJob struct {
	Target string `json:"target"`
	ID     int64  `json:"id"`
	Src    string `json:"src"`
}

// Kind implements queue.Job.
func (ThumbnailJob) Kind() string { return "catalog.thumbnails" }

// Thumbnail job targets.
const (
	ThumbProduct = "product"
	ThumbShop    = "shop"
)

// ThumbnailRegistration is the thumbnail job handler (app/jobs).
func (s *Service) ThumbnailRegistration() queue.Registration {
	return queue.Handle(s.ProcessThumbnails, queue.Retries(3), queue.Backoff(queue.Exponential(5*time.Second)))
}

// dispatch queues the variants of a new upload. A failed dispatch only
// leaves the original showing, so it is logged, not returned.
func (s *Service) dispatch(ctx context.Context, job ThumbnailJob) {
	// The upload is stored: a client that hangs up now must not abort the
	// dispatch (or, with the sync queue, the thumbnails).
	ctx = context.WithoutCancel(ctx)

	var err error
	if s.queue == nil {
		err = s.ProcessThumbnails(ctx, job)
	} else {
		err = s.queue.Dispatch(ctx, job)
	}

	if err != nil {
		s.log.WarnContext(ctx, "catalog: thumbnails", "target", job.Target, "id", job.ID, "error", err)
	}
}

func thumbOptions(width int) imaging.Options {
	return imaging.Options{
		MaxWidth: width, MaxHeight: imaging.MaxSide, MaxPixels: MaxImagePixels, MaxBytes: MaxImageBytes,
	}
}

// ProcessThumbnails is the ThumbnailJob handler. It is idempotent: done or
// stale jobs (image deleted, logo replaced) clean up and return nil.
// Images that passed the upload header checks but do not decode are
// dropped instead of retried.
func (s *Service) ProcessThumbnails(ctx context.Context, job ThumbnailJob) error {
	if _, ok := MediaObject(job.Src); !ok {
		return fmt.Errorf("catalog: thumbnail job with invalid source %q", job.Src)
	}

	switch job.Target {
	case ThumbProduct:
		return s.productThumbnails(ctx, job)
	case ThumbShop:
		return s.logoThumbnail(ctx, job)
	default:
		return fmt.Errorf("catalog: unknown thumbnail target %q", job.Target)
	}
}

func unreadable(err error) bool {
	return errors.Is(err, imaging.ErrUnsupported) || errors.Is(err, imaging.ErrTooLarge)
}

func (s *Service) productThumbnails(ctx context.Context, job ThumbnailJob) error {
	img, err := orm.QueryOf[ProductImage](s.db).Where("id", job.ID).Where("key", job.Src).Where("width", 0).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return nil // processed already, or deleted
	}

	if err != nil {
		return fmt.Errorf("load image %d: %w", job.ID, err)
	}

	sizes := make(map[string]imaging.Options, len(Widths))
	for _, w := range Widths {
		sizes["w"+strconv.Itoa(w)] = thumbOptions(w)
	}

	base := strings.TrimSuffix(img.Key, path.Ext(img.Key))

	res, err := imaging.Thumbnails(ctx, s.disk, img.Key, base, sizes)
	if unreadable(err) {
		s.log.WarnContext(ctx, "catalog: drop unreadable image", "image", img.ID, "error", err)

		if _, err := orm.QueryOf[ProductImage](s.db).Where("id", img.ID).Delete(ctx); err != nil {
			return fmt.Errorf("drop image %d: %w", img.ID, err)
		}

		s.deleteObject(ctx, img.Key)

		return nil
	}

	if err != nil {
		return fmt.Errorf("thumbnails of image %d: %w", img.ID, err)
	}

	largest := res["w"+strconv.Itoa(Widths[0])]
	key := base + path.Ext(largest.Path)

	n, err := orm.QueryOf[ProductImage](s.db).Where("id", img.ID).Where("key", img.Key).Update(ctx, orm.Set{
		"key": key, "width": largest.Width, "height": largest.Height,
	})
	if err != nil {
		return fmt.Errorf("save thumbnails of image %d: %w", img.ID, err)
	}

	if n == 0 { // deleted meanwhile
		s.deleteObjects(ctx, key, true)
	}

	s.deleteObject(ctx, img.Key)

	return nil
}

func (s *Service) logoThumbnail(ctx context.Context, job ThumbnailJob) error {
	base := strings.TrimSuffix(job.Src, path.Ext(job.Src))

	res, err := imaging.Thumbnail(ctx, s.disk, job.Src, base+"-w"+strconv.Itoa(logoWidth), thumbOptions(logoWidth))

	switch {
	case unreadable(err):
		s.log.WarnContext(ctx, "catalog: drop unreadable logo", "shop", job.ID, "error", err)

		if _, err := orm.QueryOf[Shop](s.db).Where("id", job.ID).Where("logo_key", job.Src).
			Update(ctx, orm.Set{"logo_key": nil}); err != nil {
			return fmt.Errorf("drop logo of shop %d: %w", job.ID, err)
		}

		s.deleteObject(ctx, job.Src)

		return nil
	case errors.Is(err, storage.ErrNotFound):
		return nil // replaced and deleted meanwhile
	case err != nil:
		return fmt.Errorf("thumbnail of shop %d logo: %w", job.ID, err)
	}

	// Only if the logo was not replaced meanwhile.
	n, err := orm.QueryOf[Shop](s.db).Where("id", job.ID).Where("logo_key", job.Src).
		Update(ctx, orm.Set{"logo_key": res.Path})
	if err != nil {
		return fmt.Errorf("save logo of shop %d: %w", job.ID, err)
	}

	if n == 0 {
		s.deleteObject(ctx, res.Path)
	}

	s.deleteObject(ctx, job.Src)

	return nil
}

// deleteObject removes one object; failures only leave an orphan behind,
// so they are logged, not returned.
func (s *Service) deleteObject(ctx context.Context, p string) {
	if strings.HasPrefix(p, SharedDir+"/") {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()

	if err := s.disk.Delete(ctx, p); err != nil && !errors.Is(err, storage.ErrNotFound) {
		s.log.WarnContext(ctx, "catalog: delete image object", "path", p, "error", err)
	}
}

// deleteObjects removes an image: the key itself (an unprocessed original)
// or, when processed, its variants.
func (s *Service) deleteObjects(ctx context.Context, key string, processed bool) {
	if !processed {
		s.deleteObject(ctx, key)

		return
	}

	for _, w := range Widths {
		s.deleteObject(ctx, variantPath(key, w))
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

func productDir(productID int64) string { return "products/" + strconv.FormatInt(productID, 10) }

// UploadProductImages receives the multipart field "images" of c into the
// acting user's product: ownership and free slots are checked before the
// body is read, files stream to the disk (sniffed, size and pixel capped,
// never buffered whole), and ThumbnailJob makes the variants.
//
// shortcut: the 8-image cap is checked before the insert, so two
// concurrent uploads can exceed it by one batch; a row lock or a count
// column would close that if it ever matters.
func (s *Service) UploadProductImages(c *web.Ctx, userID, productID int64) ([]ImageView, error) {
	ctx := c.Context()

	slots, err := s.ImageSlots(ctx, userID, productID)
	if err != nil {
		return nil, err
	}

	if slots == 0 {
		return nil, ErrTooManyImages
	}

	form, err := upload.Receive(c, s.disk, imageRules(productDir(productID), slots))
	if err != nil {
		return nil, err
	}

	out := make([]ImageView, 0, len(form.Files))

	for i, f := range form.Files {
		v, err := s.attachImage(ctx, productID, f.Path)
		if err != nil {
			for _, rest := range form.Files[i:] {
				s.deleteObject(ctx, rest.Path)
			}

			return nil, err
		}

		out = append(out, v)
	}

	return out, nil
}

// SaveProductImage stores one image read from r (seeders, imports) with
// the same checks as an upload.
func (s *Service) SaveProductImage(
	ctx context.Context, userID, productID int64, filename string, r io.Reader,
) (ImageView, error) {
	slots, err := s.ImageSlots(ctx, userID, productID)
	if err != nil {
		return ImageView{}, err
	}

	if slots == 0 {
		return ImageView{}, ErrTooManyImages
	}

	f, err := upload.Save(ctx, s.disk, imageRules(productDir(productID), 1), "images", filename, r)
	if err != nil {
		return ImageView{}, err
	}

	v, err := s.attachImage(ctx, productID, f.Path)
	if err != nil {
		s.deleteObject(ctx, f.Path)
	}

	return v, err
}

// attachImage appends a stored original to a product and queues its
// variants.
func (s *Service) attachImage(ctx context.Context, productID int64, key string) (ImageView, error) {
	img := ProductImage{ProductID: productID, Key: key}

	err := database.RunInTx(ctx, s.db, func(tx *database.Tx) error {
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
		return ImageView{}, fmt.Errorf("save image of product %d: %w", productID, err)
	}

	s.dispatch(ctx, ThumbnailJob{Target: ThumbProduct, ID: img.ID, Src: key})

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

	s.deleteObjects(ctx, img.Key, img.Width > 0)

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

// UploadShopLogo receives the multipart field "logo" of c as the acting
// user's shop logo; ThumbnailJob makes its thumbnail.
func (s *Service) UploadShopLogo(c *web.Ctx, userID int64) (ShopView, error) {
	ctx := c.Context()

	shop, err := s.ownShop(ctx, userID)
	if err != nil {
		return ShopView{}, err
	}

	form, err := upload.Receive(c, s.disk, imageRules("shops/"+strconv.FormatInt(shop.ID, 10), 1))
	if err != nil {
		return ShopView{}, err
	}

	key := form.Files[0].Path

	if _, err := orm.QueryOf[Shop](s.db).Where("id", shop.ID).Update(ctx, orm.Set{"logo_key": key}); err != nil {
		s.deleteObject(ctx, key)

		return ShopView{}, fmt.Errorf("set logo of shop %d: %w", shop.ID, err)
	}

	if shop.LogoKey.Valid {
		s.deleteObject(ctx, shop.LogoKey.V) // the previous logo (original or thumbnail)
	}

	shop.LogoKey.V, shop.LogoKey.Valid = key, true
	s.dispatch(ctx, ThumbnailJob{Target: ThumbShop, ID: shop.ID, Src: key})

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
