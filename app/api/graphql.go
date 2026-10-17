package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexGalhardo/vukem/contrib/graphql"
	"github.com/AlexGalhardo/vukem/errs"
)

const sdl = `
scalar DateTime

type Query {
	products(page: Int = 1, perPage: Int = 20, q: String, category: String, shop: String, sort: String): ProductPage!
	product(slug: String!): ProductDetail!
	shop(slug: String!): Shop!
	categories: [Category!]!
	me: Me!
	orders(page: Int = 1, perPage: Int = 20): OrderPage!
}

type Mutation {
	addToCart(product: String!, quantity: Int!): CartItem!
	updateCartItem(product: String!, quantity: Int!): CartItem!
	removeCartItem(product: String!): Boolean!
}

type PageMeta { page: Int! perPage: Int! total: Int! lastPage: Int! }

type Shop {
	slug: String!
	name: String!
	description: String!
	shipFromUf: String!
	products(first: Int = 10): [Product!]!
}

type Product {
	slug: String!
	title: String!
	priceCents: Int!
	currency: String!
	stock: Int!
	categorySlug: String!
	ratingAvgX100: Int!
	ratingCount: Int!
	shop: Shop!
}

type ProductPage { data: [Product!]! meta: PageMeta! }

type ProductImage { key: String! width: Int! height: Int! alt: String! }

type ProductDetail {
	slug: String!
	title: String!
	description: String!
	priceCents: Int!
	currency: String!
	stock: Int!
	weightGrams: Int!
	categorySlug: String!
	ratingAvgX100: Int!
	ratingCount: Int!
	shop: Shop!
	images: [ProductImage!]!
}

type Category { slug: String! nameEn: String! namePt: String! parentSlug: String! }

type Me { id: ID! name: String! email: String! scopes: [String!]! }

type Order {
	id: ID!
	status: String!
	paymentMethod: String!
	subtotalCents: Int!
	shippingCents: Int!
	totalCents: Int!
	currency: String!
	createdAt: DateTime!
}

type OrderPage { data: [Order!]! meta: PageMeta! }

type CartItem {
	product: String!
	title: String!
	shopSlug: String!
	priceCents: Int!
	quantity: Int!
	lineTotalCents: Int!
	available: Boolean!
}
`

// GraphQL limits: the schema is cyclic (Shop.products → Product.shop), so
// depth and complexity bound what one query can make the server do.
var graphQLLimits = graphql.Limits{
	MaxDepth:      6,
	MaxComplexity: 300,
	MaxAliases:    15,
} //nolint:gochecknoglobals // constant

type (
	productsArgs struct {
		Page     int    `json:"page"     validate:"min=1"`
		PerPage  int    `json:"perPage"  validate:"min=1,max=100"`
		Q        string `json:"q"        validate:"max=100"`
		Category string `json:"category" validate:"max=120"`
		Shop     string `json:"shop"     validate:"max=120"`
		Sort     string `json:"sort"     validate:"max=20"`
	}
	slugArgs struct {
		Slug string `json:"slug" validate:"required,max=200"`
	}
	pageArgs struct {
		Page    int `json:"page"    validate:"min=1"`
		PerPage int `json:"perPage" validate:"min=1,max=100"`
	}
	firstArgs struct {
		First int `json:"first" validate:"min=1,max=50"`
	}
	cartArgs struct {
		Product  string `json:"product"  validate:"required,max=200"`
		Quantity int    `json:"quantity" validate:"min=1,max=99"`
	}
	removeArgs struct {
		Product string `json:"product" validate:"required,max=200"`
	}
	productPage struct {
		Data []Product `json:"data"`
		Meta PageMeta  `json:"meta"`
	}
	orderPage struct {
		Data []Order  `json:"data"`
		Meta PageMeta `json:"meta"`
	}
	meResult struct {
		ID     int64    `json:"id"`
		Name   string   `json:"name"`
		Email  string   `json:"email"`
		Scopes []string `json:"scopes"`
	}
)

// NewGraphQL builds the schema over svc.
func NewGraphQL(svc Services, log *slog.Logger) (*graphql.Schema, error) {
	opts := []graphql.Option{
		graphql.WithLimits(graphQLLimits),
		graphql.WithLogger(log),
		graphql.Scalar("DateTime", graphql.ScalarFuncs{
			Serialize: func(v any) (any, error) {
				t, ok := v.(time.Time)
				if !ok {
					return nil, errors.New("DateTime: not a time")
				}

				return t.UTC().Format(time.RFC3339), nil
			},
			Parse: func(any) (any, error) { return nil, errors.New("DateTime is output only") },
		}),

		graphql.Query("products", func(ctx context.Context, a productsArgs) (productPage, error) {
			if err := allowCatalog(ctx); err != nil {
				return productPage{}, err
			}

			items, total, err := svc.Catalog.ListProducts(ctx, ProductQuery{
				Q: a.Q, Category: a.Category, Shop: a.Shop, Sort: a.Sort, Page: a.Page, PerPage: a.PerPage,
			})
			if err != nil {
				return productPage{}, HTTPError(err)
			}

			return productPage{Data: items, Meta: NewPageMeta(a.Page, a.PerPage, total)}, nil
		}),
		graphql.Query("product", func(ctx context.Context, a slugArgs) (ProductDetail, error) {
			if err := allowCatalog(ctx); err != nil {
				return ProductDetail{}, err
			}

			p, err := svc.Catalog.Product(ctx, a.Slug)

			return p, HTTPError(err)
		}),
		graphql.Query("shop", func(ctx context.Context, a slugArgs) (Shop, error) {
			if err := allowCatalog(ctx); err != nil {
				return Shop{}, err
			}

			s, err := svc.Catalog.Shop(ctx, a.Slug)

			return s, HTTPError(err)
		}),
		graphql.Query("categories", func(ctx context.Context, _ graphql.NoArgs) ([]Category, error) {
			if err := allowCatalog(ctx); err != nil {
				return nil, err
			}

			cats, err := svc.Catalog.Categories(ctx)

			return cats, HTTPError(err)
		}),
		graphql.Field("Shop", "products", func(ctx context.Context, s Shop, a firstArgs) ([]Product, error) {
			items, _, err := svc.Catalog.ListProducts(ctx, ProductQuery{Shop: s.Slug, PerPage: a.First})
			return items, HTTPError(err)
		}, graphql.Cost(5)),

		graphql.Query("me", func(ctx context.Context, _ graphql.NoArgs) (meResult, error) {
			p, err := requireScope(ctx, "")
			if err != nil {
				return meResult{}, err
			}

			me, err := svc.Profiles.Me(ctx, p.UserID)
			if err != nil {
				return meResult{}, HTTPError(err)
			}

			return meResult{ID: me.ID, Name: me.Name, Email: me.Email, Scopes: p.Scopes}, nil
		}),
		graphql.Query("orders", func(ctx context.Context, a pageArgs) (orderPage, error) {
			p, err := requireScope(ctx, ScopeOrdersRead)
			if err != nil {
				return orderPage{}, err
			}

			orders, total, err := svc.Orders.List(ctx, p.UserID, a.Page, a.PerPage)
			if err != nil {
				return orderPage{}, HTTPError(err)
			}

			return orderPage{Data: orders, Meta: NewPageMeta(a.Page, a.PerPage, total)}, nil
		}),

		graphql.Mutation("addToCart", func(ctx context.Context, a cartArgs) (CartItem, error) {
			p, err := requireScope(ctx, ScopeCartWrite)
			if err != nil {
				return CartItem{}, err
			}

			item, err := svc.Cart.Add(ctx, p.UserID, a.Product, a.Quantity)

			return item, HTTPError(err)
		}),
		graphql.Mutation("updateCartItem", func(ctx context.Context, a cartArgs) (CartItem, error) {
			p, err := requireScope(ctx, ScopeCartWrite)
			if err != nil {
				return CartItem{}, err
			}

			item, err := svc.Cart.Update(ctx, p.UserID, a.Product, a.Quantity)

			return item, HTTPError(err)
		}),
		graphql.Mutation("removeCartItem", func(ctx context.Context, a removeArgs) (bool, error) {
			p, err := requireScope(ctx, ScopeCartWrite)
			if err != nil {
				return false, err
			}

			if err := svc.Cart.Remove(ctx, p.UserID, a.Product); err != nil {
				return false, HTTPError(err)
			}

			return true, nil
		}),
	}

	return graphql.New(sdl, opts...)
}

// GraphQLHandler serves schema over HTTP. Introspection is off unless
// allowed (see GraphQLIntrospection).
func GraphQLHandler(schema *graphql.Schema, introspection bool) http.Handler {
	return schema.Handler(graphql.HandlerConfig{MaxBodyBytes: 64 << 10, DisableIntrospection: !introspection})
}

// GraphQLIntrospection is on outside production; production needs
// GRAPHQL_INSECURE_INTROSPECTION=true, since it maps the whole schema.
func GraphQLIntrospection(production bool, env string) bool {
	return !production || env == "true"
}

func allowCatalog(ctx context.Context) error {
	if p, ok := PrincipalFrom(ctx); ok && !p.Can(ScopeCatalogRead) {
		return errMissingScope(ScopeCatalogRead)
	}

	return nil
}

// requireScope needs a caller holding scope ("" accepts any caller).
func requireScope(ctx context.Context, scope string) (Principal, error) {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return Principal{}, errs.New(http.StatusUnauthorized, "A valid API key or session is required.")
	}

	if scope != "" && !p.Can(scope) {
		return Principal{}, errMissingScope(scope)
	}

	return p, nil
}
