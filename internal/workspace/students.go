package workspace

import (
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"canvasexpertse/internal/domain"
)

// Student is private machine identity. Only Ref and the chosen display name
// belong in ordinary agent packets. The store is scoped by Canvas instance.
type Student struct {
	Ref, Instance, Name, FirstName, Alias, SISID, Login string
	UserID                                              int64
	Monitored, ExtraTime                                bool
}
type StudentStore struct {
	Rows []Student
	path string
	// Choose picks one alias from the unused ones; nil takes the first. The hub
	// sets a random chooser so new students get a surprise.
	Choose func(available []string) string
}

var legacyStudentHeader = []string{"student_ref", "instance", "canvas_user_id", "name", "first_name", "alias", "sis_id", "login", "monitored"}
var studentHeader = append(append([]string(nil), legacyStudentHeader...), "extra_time")
var aliasPool = strings.Fields(`Bulbasaur Ivysaur Venusaur Charmander Charmeleon Charizard Squirtle Wartortle Blastoise Caterpie Metapod Butterfree Pidgey Pidgeotto Pidgeot Pikachu Raichu Sandshrew Sandslash Clefairy Clefable Vulpix Ninetales Jigglypuff Wigglytuff Oddish Gloom Vileplume Paras Parasect Venonat Venomoth Diglett Dugtrio Meowth Persian Psyduck Golduck Growlithe Arcanine Poliwag Poliwhirl Poliwrath Abra Kadabra Alakazam Machop Machoke Machamp Bellsprout Weepinbell Victreebel Tentacool Tentacruel Geodude Graveler Golem Ponyta Rapidash Slowpoke Slowbro Magnemite Magneton Farfetchd Doduo Dodrio Seel Dewgong Shellder Cloyster Gastly Haunter Gengar Onix Drowzee Hypno Krabby Kingler Voltorb Electrode Exeggcute Exeggutor Cubone Marowak Hitmonlee Hitmonchan Lickitung Rhyhorn Rhydon Chansey Tangela Kangaskhan Horsea Seadra Goldeen Seaking Staryu Starmie Scyther Electabuzz Magmar Pinsir Tauros Magikarp Gyarados Lapras Ditto Eevee Vaporeon Jolteon Flareon Porygon Omanyte Omastar Kabuto Kabutops Aerodactyl Snorlax Articuno Zapdos Moltres Dratini Dragonair Dragonite Mewtwo Mew Chikorita Bayleef Meganium Cyndaquil Quilava Typhlosion Totodile Croconaw Feraligatr Sentret Furret Hoothoot Noctowl Ledyba Ledian Spinarak Ariados Chinchou Lanturn Pichu Cleffa Igglybuff Togepi Togetic Natu Xatu Mareep Flaaffy Ampharos Bellossom Marill Azumarill Sudowoodo Politoed Hoppip Skiploom Jumpluff Aipom Sunkern Sunflora Yanma Wooper Quagsire Espeon Umbreon Murkrow Slowking Misdreavus Unown Wobbuffet Girafarig Pineco Forretress Dunsparce Gligar Steelix Snubbull Granbull Qwilfish Scizor Shuckle Heracross Sneasel Teddiursa Ursaring Slugma Magcargo Swinub Piloswine Corsola Remoraid Octillery Delibird Mantine Skarmory Houndour Houndoom Kingdra Phanpy Donphan Porygon2 Stantler Smeargle Tyrogue Hitmontop Smoochum Elekid Magby Miltank Blissey Raikou Entei Suicune Larvitar Pupitar Tyranitar Lugia Hooh Celebi Treecko Grovyle Sceptile Torchic Combusken Blaziken Mudkip Marshtomp Swampert Ralts Kirlia Gardevoir Aron Lairon Aggron Swablu Altaria Spheal Sealeo Walrein Bagon Shelgon Salamence Beldum Metang Metagross Turtwig Grotle Torterra Chimchar Monferno Infernape Piplup Prinplup Empoleon Shinx Luxio Luxray Riolu Lucario Leafeon Glaceon Sylveon`)

func OpenStudents(root string) (*StudentStore, error) {
	p, e := ResolveRulesPath(root, filepath.Join(".cse", "students.csv"))
	if e != nil {
		return nil, e
	}
	s := &StudentStore{path: p}
	b, e := os.ReadFile(p)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return nil, e
	}
	rows, e := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if e != nil {
		return nil, fmt.Errorf("private student table: invalid CSV")
	}
	if len(rows) == 0 || (strings.Join(rows[0], "\x00") != strings.Join(studentHeader, "\x00") && strings.Join(rows[0], "\x00") != strings.Join(legacyStudentHeader, "\x00")) {
		return nil, fmt.Errorf("private student table: unsupported header")
	}
	for i, row := range rows[1:] {
		if len(row) != len(rows[0]) {
			return nil, fmt.Errorf("private student table row %d: invalid columns", i+2)
		}
		id, e := strconv.ParseInt(row[2], 10, 64)
		if e != nil || id <= 0 {
			return nil, fmt.Errorf("private student table row %d: invalid identity", i+2)
		}
		flag, e := strconv.ParseBool(row[8])
		if e != nil {
			return nil, fmt.Errorf("private student table row %d: invalid monitored flag", i+2)
		}
		extraTime := false
		if len(row) == len(studentHeader) {
			extraTime, e = strconv.ParseBool(row[9])
			if e != nil {
				return nil, fmt.Errorf("private student table row %d: invalid extra time flag", i+2)
			}
		}
		s.Rows = append(s.Rows, Student{Ref: row[0], Instance: row[1], UserID: id, Name: row[3], FirstName: row[4], Alias: row[5], SISID: row[6], Login: row[7], Monitored: flag, ExtraTime: extraTime})
	}
	if e = s.validate(); e != nil {
		return nil, e
	}
	return s, nil
}
func (s *StudentStore) validate() error {
	refs, ids, aliases := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, v := range s.Rows {
		key := v.Instance + "/" + strconv.FormatInt(v.UserID, 10)
		aliasKey := v.Instance + "/" + strings.ToLower(v.Alias)
		if v.Ref == "" || refs[v.Ref] || v.UserID <= 0 || v.Instance == "" || ids[key] || strings.TrimSpace(v.Alias) == "" || aliases[aliasKey] {
			return fmt.Errorf("private student table contains missing or duplicate references, identities, or aliases")
		}
		if strings.ContainsAny(v.Ref, "/\\:\x00") {
			return fmt.Errorf("private student table has invalid stable reference")
		}
		refs[v.Ref] = true
		ids[key] = true
		aliases[aliasKey] = true
	}
	return nil
}
func (s *StudentStore) Save() error {
	if e := s.validate(); e != nil {
		return e
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write(studentHeader)
	for _, v := range s.Rows {
		_ = w.Write([]string{v.Ref, v.Instance, strconv.FormatInt(v.UserID, 10), v.Name, v.FirstName, v.Alias, v.SISID, v.Login, strconv.FormatBool(v.Monitored), strconv.FormatBool(v.ExtraTime)})
	}
	w.Flush()
	if e := w.Error(); e != nil {
		return e
	}
	return WriteFile(s.path, b.Bytes(), 0600)
}
func (s *StudentStore) Upsert(instance string, userID int64, name, shortName, sisID, login string) (Student, error) {
	if instance == "" || userID <= 0 {
		return Student{}, fmt.Errorf("invalid student instance or identity")
	}
	for i := range s.Rows {
		v := &s.Rows[i]
		if v.Instance == instance && v.UserID == userID {
			v.Name = name
			v.SISID = sisID
			v.Login = login
			if first := firstName(name, shortName); v.FirstName == "" && first != "" {
				v.FirstName = first
			}
			return *v, nil
		}
	}
	used := map[string]bool{}
	for _, v := range s.Rows {
		if v.Instance == instance {
			used[strings.ToLower(v.Alias)] = true
		}
	}
	alias := s.pickAlias(used, "")
	if alias == "" {
		return Student{}, fmt.Errorf("student alias pool exhausted; existing aliases are preserved")
	}
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return Student{}, e
	}
	v := Student{Ref: "s-" + hex.EncodeToString(nonce[:]), Instance: instance, UserID: userID, Name: name, FirstName: firstName(name, shortName), Alias: alias, SISID: sisID, Login: login}
	s.Rows = append(s.Rows, v)
	return v, nil
}

// pickAlias returns an unused alias from the pool, never skip or the current one.
func (s *StudentStore) pickAlias(used map[string]bool, skip string) string {
	var available []string
	for _, a := range aliasPool {
		if !used[strings.ToLower(a)] && !strings.EqualFold(a, skip) {
			available = append(available, a)
		}
	}
	switch {
	case len(available) == 0:
		return ""
	case s.Choose != nil:
		return s.Choose(available)
	}
	return available[0]
}

// Reroll gives the student a different unused alias. The stable reference does
// not change, so scoring files stay attached.
func (s *StudentStore) Reroll(ref string) (Student, error) {
	for i := range s.Rows {
		if s.Rows[i].Ref != ref {
			continue
		}
		used := map[string]bool{}
		for _, v := range s.Rows {
			if v.Instance == s.Rows[i].Instance {
				used[strings.ToLower(v.Alias)] = true
			}
		}
		alias := s.pickAlias(used, s.Rows[i].Alias)
		if alias == "" {
			return Student{}, fmt.Errorf("no unused Pokémon left to assign")
		}
		s.Rows[i].Alias = alias
		return s.Rows[i], nil
	}
	return Student{}, fmt.Errorf("student_ref %q was not found", ref)
}

func firstName(name, shortName string) string {
	if strings.TrimSpace(shortName) != "" {
		name = shortName
	}
	words := strings.Fields(name)
	if len(words) == 0 {
		return ""
	}
	return words[0]
}
func (s *StudentStore) Resolve(instance, ref string) (Student, error) {
	for _, v := range s.Rows {
		if v.Instance == instance && v.Ref == ref {
			return v, nil
		}
	}
	return Student{}, fmt.Errorf("student_ref %q is not mapped for this Canvas instance", ref)
}
func ReplaceStudentName(text string, student Student) string {
	return strings.ReplaceAll(text, "{{student_name}}", student.FirstName)
}

// Name-based notes require a unique exact name in the supplied course roster.
// Explicit references always win; fuzzy matching is deliberately absent.
func NotesFor(notes domain.StudentNotes, course string, student Student, roster []Student) ([]string, []string) {
	var result, warnings []string
	for _, n := range notes.Notes {
		if n.Course != "" && n.Course != course {
			continue
		}
		if n.StudentRef != "" {
			if n.StudentRef == student.Ref {
				result = append(result, n.Note)
			}
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(n.Name), strings.TrimSpace(student.Name)) {
			continue
		}
		count := 0
		targetMatched := false
		for _, v := range roster {
			if v.Instance == student.Instance && strings.EqualFold(strings.TrimSpace(v.Name), strings.TrimSpace(n.Name)) {
				count++
				targetMatched = targetMatched || v.Ref == student.Ref && v.UserID == student.UserID
			}
		}
		if count == 1 && targetMatched {
			result = append(result, n.Note)
		} else {
			warnings = append(warnings, "Ambiguous student note; use an explicit student_ref in context/students.md.")
		}
	}
	return result, warnings
}
