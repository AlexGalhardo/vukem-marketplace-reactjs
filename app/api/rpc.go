package api

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"slices"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"

	"github.com/AlexGalhardo/vukem"
	vgrpc "github.com/AlexGalhardo/vukem/contrib/grpc"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/router"

	catalogv1 "github.com/AlexGalhardo/vukem-marketplace-reactjs/gen/catalog/v1"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/gen/catalog/v1/catalogv1connect"
	ordersv1 "github.com/AlexGalhardo/vukem-marketplace-reactjs/gen/orders/v1"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/gen/orders/v1/ordersv1connect"
)

// RPC serves catalog.v1.CatalogService and orders.v1.OrderService
// (connect-go: gRPC, gRPC-Web and Connect) on a router of their own, the
// one the gRPC port serves, so RPC paths never reach the HTTP port.
type RPC struct {
	Svc Services
	// Middleware runs on every service route before the scope check:
	// authentication (API keys only) and rate limits.
	Middleware []router.Middleware
}

// Handler returns the RPC router: the vukem API middleware stack, the
// services, health and (outside production) reflection.
func (s *RPC) Handler(app *vukem.App) (http.Handler, error) {
	r := router.New(app.Router.Services())
	if err := vukem.APIMiddleware(app, r); err != nil {
		return nil, fmt.Errorf("rpc middleware: %w", err)
	}

	opts := vgrpc.HandlerOptions(vgrpc.Options{Log: app.Log})

	path, h := catalogv1connect.NewCatalogServiceHandler(&catalogRPC{svc: s.Svc}, opts...)
	vgrpc.Mount(r, path, h).Use(append(slices.Clone(s.Middleware), Require(ScopeCatalogRead))...)

	path, h = ordersv1connect.NewOrderServiceHandler(&ordersRPC{svc: s.Svc}, opts...)
	vgrpc.Mount(r, path, h).Use(append(slices.Clone(s.Middleware), Require(ScopeOrdersRead))...)

	vgrpc.Health(r, grpchealth.NewStaticChecker(catalogv1connect.CatalogServiceName,
		ordersv1connect.OrderServiceName))
	vgrpc.Reflection(app, r, catalogv1connect.CatalogServiceName, ordersv1connect.OrderServiceName)

	return r, nil
}

type catalogRPC struct{ svc Services }

func (c *catalogRPC) ListProducts(ctx context.Context, req *connect.Request[catalogv1.ListProductsRequest],
) (*connect.Response[catalogv1.ListProductsResponse], error) {
	page, perPage := NormalizePage(int(req.Msg.GetPage()), int(req.Msg.GetPerPage()))

	items, total, err := c.svc.Catalog.ListProducts(ctx, ProductQuery{
		Q: req.Msg.GetQ(), Category: req.Msg.GetCategory(), Shop: req.Msg.GetShop(), Sort: req.Msg.GetSort(),
		Page: page, PerPage: perPage,
	})
	if err != nil {
		return nil, HTTPError(err)
	}

	meta := NewPageMeta(page, perPage, total)
	out := &catalogv1.ListProductsResponse{Meta: &catalogv1.PageMeta{
		Page:     i32(meta.Page),
		PerPage:  i32(meta.PerPage),
		Total:    meta.Total,
		LastPage: i32(meta.LastPage),
	}}

	for i := range items {
		p := &items[i]
		out.Products = append(out.Products, productPB(p.Slug, p.Title, p.PriceCents, p.Stock, p.Shop, p.CategorySlug,
			p.RatingAvgX100, p.RatingCount))
	}

	return connect.NewResponse(out), nil
}

func (c *catalogRPC) GetProduct(ctx context.Context, req *connect.Request[catalogv1.GetProductRequest],
) (*connect.Response[catalogv1.GetProductResponse], error) {
	p, err := c.svc.Catalog.Product(ctx, req.Msg.GetSlug())
	if err != nil {
		return nil, HTTPError(err)
	}

	detail := &catalogv1.ProductDetail{
		Product: productPB(p.Slug, p.Title, p.PriceCents, p.Stock, p.Shop, p.CategorySlug,
			p.RatingAvgX100, p.RatingCount),
		Description: p.Description,
		WeightGrams: i32(p.WeightGrams),
	}

	for _, img := range p.Images {
		detail.Images = append(detail.Images, &catalogv1.ProductImage{
			Key:    img.Key,
			Width:  i32(img.Width),
			Height: i32(img.Height),
			Alt:    img.Alt,
		})
	}

	return connect.NewResponse(&catalogv1.GetProductResponse{Product: detail}), nil
}

func productPB(slug, title string, price int64, stock int, shop Shop, category string, avg, count int,
) *catalogv1.Product {
	return &catalogv1.Product{
		Slug: slug, Title: title, PriceCents: price, Currency: Currency, Stock: i32(stock),
		ShopSlug: shop.Slug, ShopName: shop.Name, CategorySlug: category,
		RatingAvgX100: i32(avg), RatingCount: i32(count),
	}
}

type ordersRPC struct{ svc Services }

func (o *ordersRPC) ListOrders(ctx context.Context, req *connect.Request[ordersv1.ListOrdersRequest],
) (*connect.Response[ordersv1.ListOrdersResponse], error) {
	p, err := requireScope(ctx, ScopeOrdersRead)
	if err != nil {
		return nil, err
	}

	page, perPage := NormalizePage(int(req.Msg.GetPage()), int(req.Msg.GetPerPage()))

	orders, total, err := o.svc.Orders.List(ctx, p.UserID, page, perPage)
	if err != nil {
		return nil, HTTPError(err)
	}

	meta := NewPageMeta(page, perPage, total)
	out := &ordersv1.ListOrdersResponse{Meta: &ordersv1.PageMeta{
		Page:     i32(meta.Page),
		PerPage:  i32(meta.PerPage),
		Total:    meta.Total,
		LastPage: i32(meta.LastPage),
	}}

	for _, ord := range orders {
		out.Orders = append(out.Orders, &ordersv1.Order{
			Id: ord.ID, Status: ord.Status, PaymentMethod: ord.PaymentMethod, SubtotalCents: ord.SubtotalCents,
			ShippingCents: ord.ShippingCents, TotalCents: ord.TotalCents, Currency: ord.Currency,
			CreatedAt: ord.CreatedAt.Format(time.RFC3339),
		})
	}

	return connect.NewResponse(out), nil
}

func (o *ordersRPC) GetOrder(
	ctx context.Context,
	req *connect.Request[ordersv1.GetOrderRequest],
) (*connect.Response[ordersv1.GetOrderResponse], error) {
	ord, err := o.order(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&ordersv1.GetOrderResponse{Order: ord}), nil
}

// WatchOrder sends the current state, then holds the stream open until the
// client leaves.
// shortcut: no live updates yet; publish from the orders:<userID> Redis
// channel once the realtime phase (O) lands.
func (o *ordersRPC) WatchOrder(ctx context.Context, req *connect.Request[ordersv1.WatchOrderRequest],
	stream *connect.ServerStream[ordersv1.WatchOrderResponse],
) error {
	ord, err := o.order(ctx, req.Msg.GetId())
	if err != nil {
		return err
	}

	if err := stream.Send(&ordersv1.WatchOrderResponse{Order: ord}); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}

func (o *ordersRPC) order(ctx context.Context, id string) (*ordersv1.Order, error) {
	p, err := requireScope(ctx, ScopeOrdersRead)
	if err != nil {
		return nil, err
	}

	if id == "" {
		return nil, errs.New(http.StatusUnprocessableEntity, "id is required")
	}

	d, err := o.svc.Orders.Get(ctx, p.UserID, id)
	if err != nil {
		return nil, HTTPError(err)
	}

	out := &ordersv1.Order{
		Id: d.ID, Status: d.Status, PaymentMethod: d.PaymentMethod, SubtotalCents: d.SubtotalCents,
		ShippingCents: d.ShippingCents, TotalCents: d.TotalCents, Currency: d.Currency,
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
	}

	for _, so := range d.SellerOrders {
		seller := &ordersv1.SellerOrder{
			ShopSlug: so.ShopSlug, ShopName: so.ShopName, Status: so.Status, SubtotalCents: so.SubtotalCents,
			ShippingCents: so.ShippingCents, Carrier: so.Carrier, TrackingCode: so.TrackingCode,
		}
		for _, it := range so.Items {
			seller.Items = append(seller.Items, &ordersv1.OrderItem{
				ProductSlug: it.ProductSlug,
				Title:       it.Title,
				PriceCents:  it.PriceCents,
				Quantity:    i32(it.Quantity),
			})
		}

		out.SellerOrders = append(out.SellerOrders, seller)
	}

	return out, nil
}

// i32 clamps n into an int32 proto field (pages, sizes and counts never
// get near the bound).
func i32(n int) int32 {
	return int32(max(min(n, math.MaxInt32), math.MinInt32))
}
