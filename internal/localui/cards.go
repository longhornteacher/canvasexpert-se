package localui

import (
	"fmt"
	"hash/fnv"
	"html/template"
	"strings"
)

// pokemonTypes name the CSS palette classes (t-<name>) in theme.go.
var pokemonTypes = []string{"normal", "fire", "water", "electric", "grass", "ice", "fighting", "poison", "ground", "flying", "psychic", "bug", "rock", "ghost", "dragon", "dark", "steel", "fairy"}

type card struct {
	Ref, Alias, Name, Course, Type string
	Level, Idx                     int
	Monitored, Shiny, Fresh        bool
	Art                            template.HTML
}

func aliasHash(alias string) uint32 {
	f := fnv.New32a()
	_, _ = f.Write([]byte(strings.ToLower(alias)))
	return f.Sum32()
}

// newCard derives everything cosmetic from the alias, so a given Pokémon always
// looks the same.
func newCard(ref, alias, name, course string, monitored, fresh bool, idx int) card {
	h := aliasHash(alias)
	return card{
		Ref: ref, Alias: alias, Name: name, Course: course, Monitored: monitored, Fresh: fresh, Idx: idx,
		Type:  pokemonTypes[int(h%uint32(len(pokemonTypes)))],
		Level: 5 + int((h>>5)%46),
		Shiny: (h>>11)%16 == 0,
		Art:   creatureArt(h),
	}
}

// creatureArt draws a small procedural monster. The markup is built only from
// integers derived from a hash, with colour supplied by CSS classes.
func creatureArt(h uint32) template.HTML {
	const cx, cy = 60, 74
	rx := 32 + int(h%9)
	ry := 28 + int((h>>4)%9)
	gap := 13 + int((h>>14)%5)
	var b strings.Builder
	b.WriteString(`<svg viewBox="0 0 120 120" role="img" aria-hidden="true">`)
	switch (h >> 8) % 4 { // ears, horns, antennae
	case 0:
		fmt.Fprintf(&b, `<polygon class="part" points="%d,%d %d,%d %d,%d"/><polygon class="part" points="%d,%d %d,%d %d,%d"/>`,
			cx-rx/2-8, cy-ry+8, cx-rx/2+2, cy-ry-24, cx-rx/2+14, cy-ry+2,
			cx+rx/2+8, cy-ry+8, cx+rx/2-2, cy-ry-24, cx+rx/2-14, cy-ry+2)
	case 1:
		fmt.Fprintf(&b, `<circle class="part" cx="%d" cy="%d" r="11"/><circle class="part" cx="%d" cy="%d" r="11"/>`, cx-rx/2, cy-ry+2, cx+rx/2, cy-ry+2)
	case 2:
		fmt.Fprintf(&b, `<path class="stalk" d="M%d,%d L%d,%d M%d,%d L%d,%d"/><circle class="tip" cx="%d" cy="%d" r="5"/><circle class="tip" cx="%d" cy="%d" r="5"/>`,
			cx-10, cy-ry+4, cx-20, cy-ry-20, cx+10, cy-ry+4, cx+20, cy-ry-20, cx-20, cy-ry-22, cx+20, cy-ry-22)
	default:
		fmt.Fprintf(&b, `<polygon class="part" points="%d,%d %d,%d %d,%d"/>`, cx-9, cy-ry+4, cx, cy-ry-26, cx+9, cy-ry+4)
	}
	switch (h >> 10) % 3 { // tails
	case 0:
		fmt.Fprintf(&b, `<path class="stalk" d="M%d,%d q26,-2 22,-28"/>`, cx+rx-6, cy+10)
	case 1:
		fmt.Fprintf(&b, `<ellipse class="part" cx="%d" cy="%d" rx="12" ry="8" transform="rotate(-25 %d %d)"/>`, cx+rx+4, cy+6, cx+rx+4, cy+6)
	default:
		fmt.Fprintf(&b, `<polyline class="bolt" points="%d,%d %d,%d %d,%d %d,%d"/>`, cx+rx-6, cy+12, cx+rx+10, cy+2, cx+rx+2, cy-6, cx+rx+18, cy-18)
	}
	fmt.Fprintf(&b, `<ellipse class="part" cx="%d" cy="%d" rx="9" ry="6"/><ellipse class="part" cx="%d" cy="%d" rx="9" ry="6"/>`, cx-rx/2, cy+ry-2, cx+rx/2, cy+ry-2)
	fmt.Fprintf(&b, `<ellipse class="body" cx="%d" cy="%d" rx="%d" ry="%d"/><ellipse class="belly" cx="%d" cy="%d" rx="%d" ry="%d"/>`, cx, cy, rx, ry, cx, cy+8, rx*6/10, ry*6/10)
	fmt.Fprintf(&b, `<circle class="cheek" cx="%d" cy="%d" r="5"/><circle class="cheek" cx="%d" cy="%d" r="5"/>`, cx-gap-13, cy+6, cx+gap+13, cy+6)
	fmt.Fprintf(&b, `<g class="eyes"><circle class="white" cx="%d" cy="%d" r="7"/><circle class="pupil" cx="%d" cy="%d" r="3.5"/><circle class="white" cx="%d" cy="%d" r="7"/><circle class="pupil" cx="%d" cy="%d" r="3.5"/></g>`,
		cx-gap, cy-6, cx-gap+1, cy-5, cx+gap, cy-6, cx+gap+1, cy-5)
	switch (h >> 12) % 3 { // mouths
	case 0:
		fmt.Fprintf(&b, `<path class="mouth" d="M%d,%d q6,7 12,0"/>`, cx-6, cy+10)
	case 1:
		fmt.Fprintf(&b, `<ellipse class="mouthfill" cx="%d" cy="%d" rx="4" ry="5"/>`, cx, cy+13)
	default:
		fmt.Fprintf(&b, `<path class="mouth" d="M%d,%d q4,5 8,0 q4,5 8,0"/>`, cx-8, cy+10)
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}
