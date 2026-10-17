package catalog

import (
	"github.com/AlexGalhardo/vukem/z"
)

// UFs are the 27 Brazilian federative units (shipping origin and filter).
var UFs = []string{
	"AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA", "MG", "MS", "MT", "PA",
	"PB", "PE", "PI", "PR", "RJ", "RN", "RO", "RR", "RS", "SC", "SE", "SP", "TO",
}

// Limits shared by the schemas and the pages.
const (
	MaxImages        = 8
	MaxPriceCents    = 100_000_000_00 // R$ 100 million
	MaxStock         = 1_000_000
	MaxWeightGrams   = 1_000_000 // 1 t; the freight table caps far below
	MaxTitleLen      = 120
	MaxDescription   = 5000
	MaxShopNameLen   = 60
	MaxShopDescLen   = 1000
	MaxSearchTermLen = 100
)

// ShopInput is the open-shop and shop-settings form.
type ShopInput struct {
	Name        string `json:"name"         form:"name"`
	Description string `json:"description"  form:"description"`
	ShipFromUF  string `json:"ship_from_uf" form:"ship_from_uf"`
}

// ShopSchema validates ShopInput.
var ShopSchema = z.Struct[ShopInput](z.Shape{
	"name":         z.String().Trim().Min(3).Max(MaxShopNameLen),
	"description":  z.String().Trim().Max(MaxShopDescLen).Optional(),
	"ship_from_uf": z.String().Trim().ToUpper().OneOf(UFs...),
})

// ProductInput is the create/edit product form. Prices are int64 cents;
// the browser converts the typed amount, the server re-validates it.
type ProductInput struct {
	Title        string `json:"title"        form:"title"`
	Description  string `json:"description"  form:"description"`
	CategorySlug string `json:"category"     form:"category"`
	PriceCents   int64  `json:"price_cents"  form:"price_cents"`
	Stock        int    `json:"stock"        form:"stock"`
	WeightGrams  int    `json:"weight_grams" form:"weight_grams"`
}

// ProductSchema validates ProductInput.
var ProductSchema = z.Struct[ProductInput](z.Shape{
	"title":        z.String().Trim().Min(3).Max(MaxTitleLen),
	"description":  z.String().Trim().Max(MaxDescription).Optional(),
	"category":     z.String().Trim().Max(64).Optional(),
	"price_cents":  z.Int64().Min(1).Max(MaxPriceCents),
	"stock":        z.Int().Min(0).Max(MaxStock),
	"weight_grams": z.Int().Min(1).Max(MaxWeightGrams),
})

// StatusInput changes a product's status.
type StatusInput struct {
	Status string `json:"status" form:"status"`
}

// StatusSchema validates StatusInput: sellers may move between draft,
// published and archived; suspended is set by moderation only.
var StatusSchema = z.Struct[StatusInput](z.Shape{
	"status": z.String().OneOf(StatusDraft, StatusPublished, StatusArchived),
})

// ReorderInput lists a product's image ids in their new order.
type ReorderInput struct {
	IDs []int64 `json:"ids"`
}

// ReorderSchema validates ReorderInput.
var ReorderSchema = z.Struct[ReorderInput](z.Shape{
	"ids": z.Slice(z.Int64().Min(1)).Min(1).Max(MaxImages).Unique(),
})
