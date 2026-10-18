package shipping

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/z"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/dbx"
)

// MaxAddresses bounds each user's address book.
const MaxAddresses = 20

// ErrTooManyAddresses is returned when the address book is full.
var ErrTooManyAddresses = errs.NewValidation().Add("recipient", "checkout.addresses.errors.too_many")

// Address is a delivery address in a user's address book.
type Address struct {
	ID         int64  `json:"id"`
	Recipient  string `json:"recipient"`
	CEP        string `json:"cep"`
	Street     string `json:"street"`
	Number     string `json:"number"`
	Complement string `json:"complement"`
	District   string `json:"district"`
	City       string `json:"city"`
	UF         string `json:"uf"`
	Phone      string `json:"phone"`
	IsDefault  bool   `json:"isDefault"`
}

// AddressInput is the address form.
type AddressInput struct {
	Recipient  string `json:"recipient"  form:"recipient"`
	CEP        string `json:"cep"        form:"cep"`
	Street     string `json:"street"     form:"street"`
	Number     string `json:"number"     form:"number"`
	Complement string `json:"complement" form:"complement"`
	District   string `json:"district"   form:"district"`
	City       string `json:"city"       form:"city"`
	UF         string `json:"uf"         form:"uf"`
	Phone      string `json:"phone"      form:"phone"`
	IsDefault  bool   `json:"isDefault"  form:"isDefault"`
}

var phoneRe = regexp.MustCompile(`^[0-9 ()+-]{8,20}$`)

// AddressSchema validates AddressInput; the UF must match the CEP's range
// so the freight region cannot be chosen apart from the real address.
var AddressSchema = z.Struct[AddressInput](z.Shape{
	"recipient":  z.String().Trim().Min(2).Max(100),
	"cep":        z.String().Trim().Max(10),
	"street":     z.String().Trim().Min(1).Max(200),
	"number":     z.String().Trim().Min(1).Max(20),
	"complement": z.String().Trim().Max(100).Optional(),
	"district":   z.String().Trim().Min(1).Max(100),
	"city":       z.String().Trim().Min(1).Max(100),
	"uf":         z.String().Trim().ToUpper().Len(2),
	"phone":      z.String().Trim().Max(20).Optional(),
	"isDefault":  z.Bool().Optional(),
}).Refine(func(in AddressInput) error {
	cep, err := NormalizeCEP(in.CEP)
	if err != nil {
		return z.FieldError("cep", "checkout.addresses.errors.cep")
	}

	uf, err := UFFromCEP(cep)
	if err != nil {
		return z.FieldError("cep", "checkout.addresses.errors.cep")
	}

	if uf != in.UF {
		return z.FieldError("uf", "checkout.addresses.errors.uf_mismatch")
	}

	if in.Phone != "" && !phoneRe.MatchString(in.Phone) {
		return z.FieldError("phone", "checkout.addresses.errors.phone")
	}

	return nil
})

// Addresses is the address book. Every query is scoped by the owner's
// user id: another user's address is simply not found (404).
type Addresses struct {
	DB  *database.DB
	Now func() time.Time
}

func (a Addresses) now() time.Time {
	if a.Now != nil {
		return a.Now().UTC()
	}

	return time.Now().UTC()
}

const addressColumns = `id, recipient, cep, street, number, complement, district, city, uf, phone, is_default`

func scanAddress(s dbx.RowScanner) (Address, error) {
	var (
		ad                Address
		complement, phone sql.NullString
	)

	err := s.Scan(&ad.ID, &ad.Recipient, &ad.CEP, &ad.Street, &ad.Number, &complement,
		&ad.District, &ad.City, &ad.UF, &phone, &ad.IsDefault)
	ad.Complement, ad.Phone = complement.String, phone.String

	return ad, err
}

// List returns the user's addresses, the default first.
func (a Addresses) List(ctx context.Context, userID int64) ([]Address, error) {
	rows, err := a.DB.QueryContext(ctx, database.Rebind(a.DB.Dialect(),
		`SELECT `+addressColumns+` FROM addresses WHERE user_id = ? ORDER BY is_default DESC, id DESC`), userID)
	if err != nil {
		return nil, fmt.Errorf("list addresses: %w", err)
	}
	defer rows.Close()

	out := []Address{}

	for rows.Next() {
		ad, err := scanAddress(rows)
		if err != nil {
			return nil, fmt.Errorf("scan address: %w", err)
		}

		out = append(out, ad)
	}

	return out, rows.Err()
}

// Get returns one of the user's addresses or orm.ErrNotFound.
func (a Addresses) Get(ctx context.Context, ex database.Executor, userID, id int64) (Address, error) {
	ad, err := scanAddress(dbx.QueryRow(ctx, ex,
		`SELECT `+addressColumns+` FROM addresses WHERE id = ? AND user_id = ?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return Address{}, orm.ErrNotFound
	}

	if err != nil {
		return Address{}, fmt.Errorf("get address %d: %w", id, err)
	}

	return ad, nil
}

func normalized(in AddressInput) (AddressInput, error) {
	cep, err := NormalizeCEP(in.CEP)
	if err != nil {
		return in, errs.NewValidation().Add("cep", "checkout.addresses.errors.cep")
	}

	in.CEP = cep

	return in, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}

	return s
}

// Create adds an address; the first one becomes the default.
func (a Addresses) Create(ctx context.Context, userID int64, in AddressInput) (Address, error) {
	in, err := normalized(in)
	if err != nil {
		return Address{}, err
	}

	var id int64

	err = a.DB.Tx(ctx, func(tx *database.Tx) error {
		var n int
		if err := dbx.QueryRow(ctx, tx, `SELECT COUNT(*) FROM addresses WHERE user_id = ?`, userID).
			Scan(&n); err != nil {
			return err
		}

		if n >= MaxAddresses {
			return ErrTooManyAddresses
		}

		isDefault := in.IsDefault || n == 0
		if isDefault {
			if _, err := dbx.Exec(
				ctx,
				tx,
				`UPDATE addresses SET is_default = ? WHERE user_id = ? AND is_default = ?`,
				false,
				userID,
				true,
			); err != nil {
				return err
			}
		}

		now := a.now()

		return dbx.QueryRow(ctx, tx,
			`INSERT INTO addresses (user_id, recipient, cep, street, number, complement, district, city, uf, phone,
				is_default, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			userID, in.Recipient, in.CEP, in.Street, in.Number, nullable(in.Complement), in.District, in.City,
			in.UF, nullable(in.Phone), isDefault, now, now).Scan(&id)
	})
	if err != nil {
		return Address{}, fmt.Errorf("create address: %w", err)
	}

	return a.Get(ctx, a.DB, userID, id)
}

// Update changes one of the user's addresses.
func (a Addresses) Update(ctx context.Context, userID, id int64, in AddressInput) (Address, error) {
	in, err := normalized(in)
	if err != nil {
		return Address{}, err
	}

	err = a.DB.Tx(ctx, func(tx *database.Tx) error {
		current, err := a.Get(ctx, tx, userID, id)
		if err != nil {
			return err
		}

		isDefault := current.IsDefault || in.IsDefault
		if isDefault && !current.IsDefault {
			if _, err := dbx.Exec(
				ctx,
				tx,
				`UPDATE addresses SET is_default = ? WHERE user_id = ? AND is_default = ?`,
				false,
				userID,
				true,
			); err != nil {
				return err
			}
		}

		_, err = dbx.Exec(ctx, tx,
			`UPDATE addresses SET recipient = ?, cep = ?, street = ?, number = ?, complement = ?, district = ?,
				city = ?, uf = ?, phone = ?, is_default = ?, updated_at = ?
			WHERE id = ? AND user_id = ?`,
			in.Recipient, in.CEP, in.Street, in.Number, nullable(in.Complement), in.District, in.City, in.UF,
			nullable(in.Phone), isDefault, a.now(), id, userID)

		return err
	})
	if err != nil {
		return Address{}, fmt.Errorf("update address %d: %w", id, err)
	}

	return a.Get(ctx, a.DB, userID, id)
}

// SetDefault makes one of the user's addresses the default.
func (a Addresses) SetDefault(ctx context.Context, userID, id int64) error {
	return a.DB.Tx(ctx, func(tx *database.Tx) error {
		if _, err := a.Get(ctx, tx, userID, id); err != nil {
			return err
		}

		if _, err := dbx.Exec(
			ctx,
			tx,
			`UPDATE addresses SET is_default = ? WHERE user_id = ? AND is_default = ?`,
			false,
			userID,
			true,
		); err != nil {
			return err
		}

		_, err := dbx.Exec(ctx, tx, `UPDATE addresses SET is_default = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
			true, a.now(), id, userID)

		return err
	})
}

// Delete removes one of the user's addresses; when it was the default, the
// newest remaining address takes over. Orders keep their own snapshot.
func (a Addresses) Delete(ctx context.Context, userID, id int64) error {
	return a.DB.Tx(ctx, func(tx *database.Tx) error {
		current, err := a.Get(ctx, tx, userID, id)
		if err != nil {
			return err
		}

		if _, err := dbx.Exec(ctx, tx, `DELETE FROM addresses WHERE id = ? AND user_id = ?`, id, userID); err != nil {
			return err
		}

		if !current.IsDefault {
			return nil
		}

		_, err = dbx.Exec(ctx, tx,
			`UPDATE addresses SET is_default = ? WHERE id = (SELECT MAX(id) FROM addresses WHERE user_id = ?)`,
			true, userID)

		return err
	})
}
