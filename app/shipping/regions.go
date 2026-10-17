// Package shipping prices delivery and keeps buyers' addresses: the ViaCEP
// client (address autofill), the CEP → UF → region maps, the simulated
// freight table (weight band × destination region) and the address book.
package shipping

import (
	"errors"
	"strconv"
	"strings"
)

// Region is a destination region of Brazil, the freight table's key.
type Region string

// The five regions (IBGE).
const (
	North      Region = "N"
	Northeast  Region = "NE"
	CenterWest Region = "CO"
	Southeast  Region = "SE"
	South      Region = "S"
)

// Regions lists the regions in a stable order.
func Regions() []Region { return []Region{North, Northeast, CenterWest, Southeast, South} }

var ufRegion = map[string]Region{
	"AC": North, "AM": North, "AP": North, "PA": North, "RO": North, "RR": North, "TO": North,
	"AL": Northeast, "BA": Northeast, "CE": Northeast, "MA": Northeast, "PB": Northeast,
	"PE": Northeast, "PI": Northeast, "RN": Northeast, "SE": Northeast,
	"DF": CenterWest, "GO": CenterWest, "MS": CenterWest, "MT": CenterWest,
	"ES": Southeast, "MG": Southeast, "RJ": Southeast, "SP": Southeast,
	"PR": South, "RS": South, "SC": South,
}

// ErrInvalidUF is returned for an unknown state code.
var ErrInvalidUF = errors.New("shipping: unknown UF")

// ErrInvalidCEP is returned for a CEP that is not 8 digits or falls in no
// state's range.
var ErrInvalidCEP = errors.New("shipping: invalid CEP")

// RegionOf returns the region of a state code (UF).
func RegionOf(uf string) (Region, error) {
	r, ok := ufRegion[strings.ToUpper(strings.TrimSpace(uf))]
	if !ok {
		return "", ErrInvalidUF
	}

	return r, nil
}

// ValidUF reports whether uf is one of the 27 state codes.
func ValidUF(uf string) bool {
	_, ok := ufRegion[uf]
	return ok
}

// NormalizeCEP strips the usual "01310-100" punctuation and checks the CEP
// is exactly 8 digits.
func NormalizeCEP(raw string) (string, error) {
	var b strings.Builder

	for _, r := range strings.TrimSpace(raw) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '.' || r == ' ':
		default:
			return "", ErrInvalidCEP
		}
	}

	cep := b.String()
	if len(cep) != 8 {
		return "", ErrInvalidCEP
	}

	return cep, nil
}

// cepRange maps the first five CEP digits to a UF (Correios ranges).
type cepRange struct {
	from, to int
	uf       string
}

var cepRanges = []cepRange{
	{1000, 19999, "SP"},
	{20000, 28999, "RJ"},
	{29000, 29999, "ES"},
	{30000, 39999, "MG"},
	{40000, 48999, "BA"},
	{49000, 49999, "SE"},
	{50000, 56999, "PE"},
	{57000, 57999, "AL"},
	{58000, 58999, "PB"},
	{59000, 59999, "RN"},
	{60000, 63999, "CE"},
	{64000, 64999, "PI"},
	{65000, 65999, "MA"},
	{66000, 68899, "PA"},
	{68900, 68999, "AP"},
	{69000, 69299, "AM"},
	{69300, 69399, "RR"},
	{69400, 69899, "AM"},
	{69900, 69999, "AC"},
	{70000, 72799, "DF"},
	{72800, 72999, "GO"},
	{73000, 73699, "DF"},
	{73700, 76799, "GO"},
	{76800, 76999, "RO"},
	{77000, 77999, "TO"},
	{78000, 78899, "MT"},
	{79000, 79999, "MS"},
	{80000, 87999, "PR"},
	{88000, 89999, "SC"},
	{90000, 99999, "RS"},
}

// UFFromCEP returns the state of a CEP from the Correios ranges, offline,
// so freight quotes never depend on ViaCEP being up.
func UFFromCEP(raw string) (string, error) {
	cep, err := NormalizeCEP(raw)
	if err != nil {
		return "", err
	}

	prefix, err := strconv.Atoi(cep[:5])
	if err != nil {
		return "", ErrInvalidCEP
	}

	for _, r := range cepRanges {
		if prefix >= r.from && prefix <= r.to {
			return r.uf, nil
		}
	}

	return "", ErrInvalidCEP
}
