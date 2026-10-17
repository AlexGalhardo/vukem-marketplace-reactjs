package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/database"
)

// API keys look like mkt_<prefix>_<secret>. The prefix is the public
// lookup id (shown in the account page); only the SHA-256 of the secret is
// stored, so a database leak does not leak usable keys.
const (
	TokenPrefix   = "mkt_"
	prefixLen     = 12
	secretLen     = 43 // base62 characters: about 256 bits
	minPrefixLen  = 8
	maxPrefixLen  = 16
	MaxActiveKeys = 20
	maxNameLen    = 60
	// touchEvery throttles last_used_at writes to one per key per minute.
	touchEvery = time.Minute
)

// Key errors.
var (
	// ErrInvalidKey is every authentication failure (unknown prefix, wrong
	// secret, malformed, revoked, expired): one error, so responses never
	// tell which part was wrong.
	ErrInvalidKey = errors.New("api: invalid api key")
	// ErrTooManyKeys is returned when the user already has MaxActiveKeys.
	ErrTooManyKeys = errors.New("api: too many active api keys")
	// ErrKeyNotFound covers missing and other users' keys.
	ErrKeyNotFound = errors.New("api: api key not found")
)

// Key is a stored API key (never its secret).
type Key struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"-"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`

	secretHash string
}

// Active reports whether k can authenticate at now.
func (k Key) Active(now time.Time) bool {
	return k.RevokedAt == nil && (k.ExpiresAt == nil || now.Before(*k.ExpiresAt))
}

// Can reports whether k holds scope.
func (k Key) Can(scope string) bool { return slices.Contains(k.Scopes, scope) }

// NewKey is the input of Keys.Create.
type NewKey struct {
	Name      string
	Scopes    []string
	ExpiresAt *time.Time
}

// Keys stores and verifies API keys.
type Keys struct {
	db  *database.DB
	now func() time.Time
}

// NewKeys returns the key store over db; now is the app clock.
func NewKeys(db *database.DB, now func() time.Time) *Keys {
	return &Keys{db: db, now: now}
}

const keyColumns = `id, user_id, name, prefix, secret_hash, scopes, last_used_at, expires_at, revoked_at, created_at`

// Create stores a new key for userID and returns it with the plain token,
// which is shown once and never stored.
func (k *Keys) Create(ctx context.Context, userID int64, in NewKey) (Key, string, error) {
	scopes := normalizeScopes(in.Scopes)
	name := strings.TrimSpace(in.Name)

	switch {
	case name == "" || len([]rune(name)) > maxNameLen:
		return Key{}, "", &InputError{Field: "name"}
	case len(scopes) == 0 || len(scopes) != len(slices.Compact(slices.Sorted(slices.Values(in.Scopes)))):
		return Key{}, "", &InputError{Field: "scopes"}
	}

	now := k.now().UTC()
	if in.ExpiresAt != nil && !in.ExpiresAt.After(now) {
		return Key{}, "", &InputError{Field: "expires_at"}
	}

	prefix, secret := randomBase62(prefixLen), randomBase62(secretLen)
	key := Key{
		UserID: userID, Name: name, Prefix: prefix, Scopes: scopes,
		ExpiresAt: in.ExpiresAt, CreatedAt: now, secretHash: hashSecret(secret),
	}

	err := k.db.Tx(ctx, func(tx *database.Tx) error {
		// Serialise concurrent creates of one user so the cap holds
		// (SQLite has a single writer already).
		if tx.Dialect().Name() == "postgres" {
			if _, err := tx.ExecContext(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID); err != nil {
				return fmt.Errorf("lock user %d: %w", userID, err)
			}
		}

		keys, err := k.list(ctx, tx, userID)
		if err != nil {
			return err
		}

		active := 0
		for i := range keys {
			if keys[i].Active(now) {
				active++
			}
		}

		if active >= MaxActiveKeys {
			return ErrTooManyKeys
		}

		return tx.QueryRowContext(ctx, database.Rebind(tx.Dialect(),
			`INSERT INTO api_keys (user_id, name, prefix, secret_hash, scopes, expires_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`),
			userID, name, prefix, key.secretHash, strings.Join(scopes, " "), nullTime(in.ExpiresAt), now, now,
		).Scan(&key.ID)
	})
	if err != nil {
		return Key{}, "", err
	}

	return key, TokenPrefix + prefix + "_" + secret, nil
}

// List returns userID's keys, newest first (revoked and expired ones too).
func (k *Keys) List(ctx context.Context, userID int64) ([]Key, error) {
	return k.list(ctx, k.db, userID)
}

func (k *Keys) list(ctx context.Context, ex database.Executor, userID int64) ([]Key, error) {
	rows, err := ex.QueryContext(ctx, database.Rebind(ex.Dialect(),
		`SELECT `+keyColumns+` FROM api_keys WHERE user_id = ? ORDER BY id DESC`), userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys of user %d: %w", userID, err)
	}
	defer rows.Close()

	var keys []Key

	for rows.Next() {
		key, err := scanKey(rows)
		if err != nil {
			return nil, err
		}

		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// Revoke revokes userID's key id. Other users' keys are ErrKeyNotFound,
// exactly like missing ones; revoking twice is a no-op.
func (k *Keys) Revoke(ctx context.Context, userID, id int64) error {
	var revoked sql.NullTime

	err := k.db.QueryRowContext(ctx, database.Rebind(k.db.Dialect(),
		`SELECT revoked_at FROM api_keys WHERE id = ? AND user_id = ?`), id, userID).Scan(&revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrKeyNotFound
	}

	if err != nil {
		return fmt.Errorf("find api key %d: %w", id, err)
	}

	if revoked.Valid {
		return nil
	}

	now := k.now().UTC()

	_, err = k.db.ExecContext(ctx, database.Rebind(k.db.Dialect(),
		`UPDATE api_keys SET revoked_at = ?, updated_at = ? WHERE id = ? AND user_id = ? AND revoked_at IS NULL`),
		now, now, id, userID)
	if err != nil {
		return fmt.Errorf("revoke api key %d: %w", id, err)
	}

	return nil
}

// dummyHash is compared when the prefix is unknown so that path costs the
// same as a wrong secret.
var dummyHash = hashSecret(strings.Repeat("0", secretLen)) //nolint:gochecknoglobals // constant digest

// Authenticate verifies a plain token and returns its key.
func (k *Keys) Authenticate(ctx context.Context, token string) (Key, error) {
	prefix, secret, ok := splitToken(token)
	if !ok {
		subtle.ConstantTimeCompare([]byte(dummyHash), []byte(hashSecret(secret)))
		return Key{}, ErrInvalidKey
	}

	row := k.db.QueryRowContext(ctx, database.Rebind(k.db.Dialect(),
		`SELECT `+keyColumns+` FROM api_keys WHERE prefix = ?`), prefix)

	key, err := scanKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		subtle.ConstantTimeCompare([]byte(dummyHash), []byte(hashSecret(secret)))
		return Key{}, ErrInvalidKey
	}

	if err != nil {
		return Key{}, fmt.Errorf("find api key: %w", err)
	}

	match := subtle.ConstantTimeCompare([]byte(key.secretHash), []byte(hashSecret(secret))) == 1
	if !match || !key.Active(k.now()) {
		return Key{}, ErrInvalidKey
	}

	return key, nil
}

// Touch records that key was used, at most once per touchEvery.
func (k *Keys) Touch(ctx context.Context, key Key) error {
	now := k.now().UTC()
	if key.LastUsedAt != nil && now.Sub(*key.LastUsedAt) < touchEvery {
		return nil
	}

	_, err := k.db.ExecContext(ctx, database.Rebind(k.db.Dialect(),
		`UPDATE api_keys SET last_used_at = ? WHERE id = ?`), now, key.ID)
	if err != nil {
		return fmt.Errorf("touch api key %d: %w", key.ID, err)
	}

	return nil
}

// splitToken parses mkt_<prefix>_<secret>. It returns the secret part even
// when the token is malformed so the caller can still hash something.
func splitToken(token string) (prefix, secret string, ok bool) {
	rest, found := strings.CutPrefix(token, TokenPrefix)
	if !found {
		return "", token, false
	}

	prefix, secret, found = strings.Cut(rest, "_")
	if !found || len(prefix) < minPrefixLen || len(prefix) > maxPrefixLen || len(secret) != secretLen ||
		!isBase62(prefix) || !isBase62(secret) {
		return "", secret, false
	}

	return prefix, secret, true
}

type scanner interface{ Scan(dest ...any) error }

func scanKey(s scanner) (Key, error) {
	var (
		key                       Key
		scopes                    string
		lastUsed, expires, revoke sql.NullTime
	)

	err := s.Scan(&key.ID, &key.UserID, &key.Name, &key.Prefix, &key.secretHash, &scopes,
		&lastUsed, &expires, &revoke, &key.CreatedAt)
	if err != nil {
		return Key{}, err
	}

	key.Scopes = parseScopes(scopes)
	key.LastUsedAt, key.ExpiresAt, key.RevokedAt = timePtr(lastUsed), timePtr(expires), timePtr(revoke)

	return key, nil
}

func timePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}

	return new(t.Time.UTC())
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}

	return sql.NullTime{Time: t.UTC(), Valid: true}
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// randomBase62 returns n uniformly random base62 characters from
// crypto/rand (rejection sampling avoids modulo bias).
func randomBase62(n int) string {
	out := make([]byte, 0, n)
	buf := make([]byte, n*2)

	for len(out) < n {
		_, _ = rand.Read(buf) // never fails (crypto/rand panics instead)

		for _, b := range buf {
			if b >= 248 { // 248 = 62*4: keep only unbiased bytes
				continue
			}

			out = append(out, base62[b%62])
			if len(out) == n {
				break
			}
		}
	}

	return string(out)
}

func isBase62(s string) bool {
	for i := range len(s) {
		if !strings.ContainsRune(base62, rune(s[i])) {
			return false
		}
	}

	return true
}
