// Package seller serves the "open a shop" flow and the seller dashboard.
// Every handler acts as the signed-in user; the catalog service scopes each
// read and write to that user's shop, so other sellers' ids answer 404.
package seller

import (
	"errors"
	"strconv"

	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/storage"
	"github.com/AlexGalhardo/vukem/upload"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
)

// Handlers serves the seller pages.
type Handlers struct {
	svc *catalog.Service
	acc *accounts.Accounts
}

// New returns the handlers.
func New(svc *catalog.Service, acc *accounts.Accounts) *Handlers {
	return &Handlers{svc: svc, acc: acc}
}

// uid is the acting user (the routes require a signed-in, verified user).
func (h *Handlers) uid(c *web.Ctx) (int64, error) {
	u, err := h.acc.Web.User(c)
	if err != nil {
		return 0, err
	}

	return u.ID, nil
}

// noShop sends users without a shop to /sell; other errors pass through.
func noShop(c *web.Ctx, err error) error {
	if errors.Is(err, catalog.ErrNoShop) {
		return c.Redirect("/sell")
	}

	return err
}

func productID(c *web.Ctx) (int64, error) {
	id, err := c.ParamInt("id")
	if err != nil || id <= 0 {
		return 0, catalog.ErrNotFound
	}

	return id, nil
}

// ShowOpen renders the open-a-shop form (or goes to the dashboard).
func (h *Handlers) ShowOpen(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	if _, err := h.svc.MyShop(c.Context(), uid); err == nil {
		return c.Redirect("/seller")
	} else if !errors.Is(err, catalog.ErrNoShop) {
		return err
	}

	return inertia.Render(c, "Seller/Open", inertia.Props{"ufs": catalog.UFs})
}

// Open creates the shop.
func (h *Handlers) Open(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, catalog.ShopSchema)
	if err != nil {
		return err
	}

	if _, err := h.svc.OpenShop(c.Context(), uid, in); err != nil {
		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "seller.flash.shop_opened"))

	return c.Redirect("/seller")
}

// Dashboard renders the seller home.
func (h *Handlers) Dashboard(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	shop, err := h.svc.MyShop(c.Context(), uid)
	if err != nil {
		return noShop(c, err)
	}

	products, err := h.svc.SellerProducts(c.Context(), uid, 1)
	if err != nil {
		return err
	}

	return inertia.Render(c, "Seller/Dashboard", inertia.Props{"shop": shop, "products": products})
}

// Settings renders the shop branding form.
func (h *Handlers) Settings(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	shop, err := h.svc.MyShop(c.Context(), uid)
	if err != nil {
		return noShop(c, err)
	}

	return inertia.Render(c, "Seller/Settings", inertia.Props{"shop": shop, "ufs": catalog.UFs})
}

// UpdateSettings saves the shop branding.
func (h *Handlers) UpdateSettings(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, catalog.ShopSchema)
	if err != nil {
		return err
	}

	if _, err := h.svc.UpdateShop(c.Context(), uid, in); err != nil {
		return noShop(c, err)
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "seller.flash.saved"))

	return c.Redirect("/seller/settings")
}

// UploadLogo replaces the shop logo.
func (h *Handlers) UploadLogo(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	if _, err := h.svc.MyShop(c.Context(), uid); err != nil {
		return noShop(c, err)
	}

	scratch := storage.NewMemory("")

	form, err := upload.Receive(c, scratch, catalog.ImageRules(1))
	if err != nil {
		return err
	}

	rc, err := scratch.Open(c.Context(), form.Files[0].Path)
	if err != nil {
		return err
	}
	defer rc.Close()

	if _, err := h.svc.SetShopLogo(c.Context(), uid, rc); err != nil {
		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "seller.flash.saved"))

	return c.Redirect("/seller/settings")
}

// Products lists the seller's products.
func (h *Handlers) Products(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	page, err := h.svc.SellerProducts(c.Context(), uid, c.QueryInt("page", 1))
	if err != nil {
		return noShop(c, err)
	}

	return inertia.Render(c, "Seller/Products", inertia.Props{"products": page})
}

func (h *Handlers) formProps(c *web.Ctx, product any) (inertia.Props, error) {
	cats, err := h.svc.Categories(c.Context())
	if err != nil {
		return nil, err
	}

	return inertia.Props{
		"product":    product,
		"categories": catalog.LocalizedCategories(cats, i18n.Locale(c.Context())),
		"maxImages":  catalog.MaxImages,
	}, nil
}

// NewProduct renders the create form.
func (h *Handlers) NewProduct(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	if _, err := h.svc.MyShop(c.Context(), uid); err != nil {
		return noShop(c, err)
	}

	props, err := h.formProps(c, nil)
	if err != nil {
		return err
	}

	return inertia.Render(c, "Seller/ProductForm", props)
}

// CreateProduct saves a draft and continues to its edit page (images).
func (h *Handlers) CreateProduct(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, catalog.ProductSchema)
	if err != nil {
		return err
	}

	p, err := h.svc.CreateProduct(c.Context(), uid, in)
	if err != nil {
		return noShop(c, err)
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "seller.flash.product_created"))

	return c.Redirect(editURL(p.ID))
}

func editURL(id int64) string { return "/seller/products/" + strconv.FormatInt(id, 10) + "/edit" }

// EditProduct renders the edit form.
func (h *Handlers) EditProduct(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	id, err := productID(c)
	if err != nil {
		return err
	}

	p, err := h.svc.SellerProduct(c.Context(), uid, id)
	if err != nil {
		return err
	}

	props, err := h.formProps(c, p)
	if err != nil {
		return err
	}

	return inertia.Render(c, "Seller/ProductForm", props)
}

// UpdateProduct saves the edit form.
func (h *Handlers) UpdateProduct(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	id, err := productID(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, catalog.ProductSchema)
	if err != nil {
		return err
	}

	if _, err := h.svc.UpdateProduct(c.Context(), uid, id, in); err != nil {
		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "seller.flash.saved"))

	return c.Redirect(editURL(id))
}

// SetStatus publishes, unpublishes (draft) or archives a product.
func (h *Handlers) SetStatus(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	id, err := productID(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, catalog.StatusSchema)
	if err != nil {
		return err
	}

	if _, err := h.svc.SetProductStatus(c.Context(), uid, id, in.Status); err != nil {
		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "seller.flash.saved"))

	return c.Back(editURL(id))
}

// UploadImages adds images to a product (multipart field "images").
// Ownership and the free slots are checked before the body is read.
func (h *Handlers) UploadImages(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	id, err := productID(c)
	if err != nil {
		return err
	}

	slots, err := h.svc.ImageSlots(c.Context(), uid, id)
	if err != nil {
		return err
	}

	if slots == 0 {
		return catalog.ErrTooManyImages
	}

	// Files are streamed to memory (bounded by the rules), decoded and
	// re-encoded; only the resized variants reach the bucket.
	scratch := storage.NewMemory("")

	form, err := upload.Receive(c, scratch, catalog.ImageRules(slots))
	if err != nil {
		return err
	}

	for _, f := range form.Files {
		if err := h.addImage(c, scratch, uid, id, f); err != nil {
			return err
		}
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "seller.flash.images_added"))

	return c.Redirect(editURL(id))
}

func (h *Handlers) addImage(c *web.Ctx, scratch storage.Disk, uid, id int64, f upload.File) error {
	rc, err := scratch.Open(c.Context(), f.Path)
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = h.svc.AddProductImage(c.Context(), uid, id, rc, "")

	return err
}

// ReorderImages saves a new image order.
func (h *Handlers) ReorderImages(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	id, err := productID(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, catalog.ReorderSchema)
	if err != nil {
		return err
	}

	if err := h.svc.ReorderProductImages(c.Context(), uid, id, in.IDs); err != nil {
		return err
	}

	return c.Redirect(editURL(id))
}

// DeleteImage removes one image.
func (h *Handlers) DeleteImage(c *web.Ctx) error {
	uid, err := h.uid(c)
	if err != nil {
		return err
	}

	id, err := productID(c)
	if err != nil {
		return err
	}

	imageID, err := c.ParamInt("image")
	if err != nil {
		return catalog.ErrNotFound
	}

	if err := h.svc.DeleteProductImage(c.Context(), uid, id, imageID); err != nil {
		return err
	}

	return c.Redirect(editURL(id))
}
