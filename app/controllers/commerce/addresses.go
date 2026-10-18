package commerce

import (
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/shipping"
)

// ShowAddresses renders the address book.
func (h *Handlers) ShowAddresses(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	list, err := h.Addresses.List(c.Context(), u.ID)
	if err != nil {
		return err
	}

	return inertia.Render(c, "Account/Addresses", inertia.Props{"addresses": list, "max": shipping.MaxAddresses})
}

// StoreAddress adds an address.
func (h *Handlers) StoreAddress(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, shipping.AddressSchema)
	if err != nil {
		return err
	}

	if _, err := h.Addresses.Create(c.Context(), u.ID, in); err != nil {
		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "checkout.addresses.flash.saved"))

	return c.Back("/account/addresses")
}

// UpdateAddress changes one of the user's addresses (404 otherwise).
func (h *Handlers) UpdateAddress(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := paramID(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, shipping.AddressSchema)
	if err != nil {
		return err
	}

	if _, err := h.Addresses.Update(c.Context(), u.ID, id, in); err != nil {
		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "checkout.addresses.flash.saved"))

	return c.Redirect("/account/addresses")
}

// DestroyAddress deletes one of the user's addresses.
func (h *Handlers) DestroyAddress(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := paramID(c)
	if err != nil {
		return err
	}

	if err := h.Addresses.Delete(c.Context(), u.ID, id); err != nil {
		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "checkout.addresses.flash.deleted"))

	return c.Redirect("/account/addresses")
}

// DefaultAddress makes one of the user's addresses the default.
func (h *Handlers) DefaultAddress(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := paramID(c)
	if err != nil {
		return err
	}

	if err := h.Addresses.SetDefault(c.Context(), u.ID, id); err != nil {
		return err
	}

	return c.Back("/account/addresses")
}
