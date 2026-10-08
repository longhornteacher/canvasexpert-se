package domain

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Decimal stores the exact supplied decimal spelling. Its zero value is numeric
// zero; optional scores must use *Decimal to distinguish zero from absence.
type Decimal struct{ lexeme string }

var decimalPattern = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)$`)

func ParseDecimal(s string) (Decimal, error) {
	if !decimalPattern.MatchString(s) || len(s) > 256 {
		return Decimal{}, fmt.Errorf("invalid finite decimal %q", s)
	}
	if _, ok := new(big.Rat).SetString(s); !ok {
		return Decimal{}, fmt.Errorf("invalid decimal %q", s)
	}
	return Decimal{lexeme: s}, nil
}
func MustDecimal(s string) Decimal {
	d, err := ParseDecimal(s)
	if err != nil {
		panic(err)
	}
	return d
}
func (d Decimal) String() string {
	if d.lexeme == "" {
		return "0"
	}
	return d.lexeme
}
func (d Decimal) Rat() *big.Rat             { r, _ := new(big.Rat).SetString(d.String()); return r }
func (d Decimal) Cmp(other Decimal) int     { return d.Rat().Cmp(other.Rat()) }
func (d Decimal) Sign() int                 { return d.Rat().Sign() }
func (d Decimal) IsSet() bool               { return d.lexeme != "" }
func (d Decimal) MarshalYAML() (any, error) { return d.String(), nil }
func (d *Decimal) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return fmt.Errorf("line %d: expected finite decimal", n.Line)
	}
	v, err := ParseDecimal(n.Value)
	if err != nil {
		return fmt.Errorf("line %d: %w", n.Line, err)
	}
	*d = v
	return nil
}
func (d Decimal) MarshalJSON() ([]byte, error) {
	// A valid input decimal may use +, leading zeros, or .5, which JSON
	// numbers prohibit. Keep its precision while emitting a legal number.
	places := 0
	if at := strings.IndexByte(d.String(), '.'); at >= 0 {
		places = len(d.String()) - at - 1
	}
	return []byte(d.Rat().FloatString(places)), nil
}
func (d *Decimal) UnmarshalJSON(b []byte) error {
	s := string(b)
	if strings.HasPrefix(s, `"`) {
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
	}
	v, err := ParseDecimal(s)
	if err != nil {
		return err
	}
	*d = v
	return nil
}

// RoundRat rounds half away from zero only at a stated display/posting boundary.
func RoundRat(r *big.Rat, places int) Decimal {
	if places < 0 || places > 20 {
		panic("decimal precision out of range")
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	n := new(big.Int).Mul(r.Num(), scale)
	abs := new(big.Int).Abs(n)
	q, rem := new(big.Int).QuoRem(abs, r.Denom(), new(big.Int))
	if new(big.Int).Mul(rem, big.NewInt(2)).Cmp(r.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	s := q.String()
	if places > 0 {
		for len(s) <= places {
			s = "0" + s
		}
		s = s[:len(s)-places] + "." + s[len(s)-places:]
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if n.Sign() < 0 && q.Sign() != 0 {
		s = "-" + s
	}
	return MustDecimal(s)
}
