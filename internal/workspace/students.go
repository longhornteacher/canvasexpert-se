package workspace

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	// Quiz settings are private; ExtraTime above remains a deadline flag.
	QuizTime       string `json:"-" yaml:"-"`
	ReducedChoices bool   `json:"-" yaml:"-"`
	// AliasSource is AliasFromCanvasID for a name every install derives the
	// same way, or AliasFromTeacher for one the teacher typed.
	AliasSource string
}
type StudentStore struct {
	Rows []Student
	path string
}

const (
	AliasFromCanvasID = "canvas_id"
	AliasFromTeacher  = "teacher"
)

var legacyStudentHeader = []string{"student_ref", "instance", "canvas_user_id", "name", "first_name", "alias", "sis_id", "login", "monitored"}
var extraTimeStudentHeader = append(append([]string(nil), legacyStudentHeader...), "extra_time")
var aliasSourceStudentHeader = append(append([]string(nil), extraTimeStudentHeader...), "alias_source")
var studentHeader = append(append([]string(nil), aliasSourceStudentHeader...), "quiz_time", "reduced_choices")

// aliasPool is frozen: aliases are derived from the Canvas identity by position
// in this list and by its length, so any change to it would give students
// different names on a fresh install.
var aliasPool = strings.Fields(`Bulbasaur Ivysaur Venusaur Charmander Charmeleon Charizard Squirtle Wartortle Blastoise Caterpie Metapod Butterfree Pidgey Pidgeotto Pidgeot Pikachu Raichu Sandshrew Sandslash Clefairy Clefable Vulpix Ninetales Jigglypuff Wigglytuff Oddish Gloom Vileplume Paras Parasect Venonat Venomoth Diglett Dugtrio Meowth Persian Psyduck Golduck Growlithe Arcanine Poliwag Poliwhirl Poliwrath Abra Kadabra Alakazam Machop Machoke Machamp Bellsprout Weepinbell Victreebel Tentacool Tentacruel Geodude Graveler Golem Ponyta Rapidash Slowpoke Slowbro Magnemite Magneton Farfetchd Doduo Dodrio Seel Dewgong Shellder Cloyster Gastly Haunter Gengar Onix Drowzee Hypno Krabby Kingler Voltorb Electrode Exeggcute Exeggutor Cubone Marowak Hitmonlee Hitmonchan Lickitung Rhyhorn Rhydon Chansey Tangela Kangaskhan Horsea Seadra Goldeen Seaking Staryu Starmie Scyther Electabuzz Magmar Pinsir Tauros Magikarp Gyarados Lapras Ditto Eevee Vaporeon Jolteon Flareon Porygon Omanyte Omastar Kabuto Kabutops Aerodactyl Snorlax Articuno Zapdos Moltres Dratini Dragonair Dragonite Mewtwo Mew Chikorita Bayleef Meganium Cyndaquil Quilava Typhlosion Totodile Croconaw Feraligatr Sentret Furret Hoothoot Noctowl Ledyba Ledian Spinarak Ariados Chinchou Lanturn Pichu Cleffa Igglybuff Togepi Togetic Natu Xatu Mareep Flaaffy Ampharos Bellossom Marill Azumarill Sudowoodo Politoed Hoppip Skiploom Jumpluff Aipom Sunkern Sunflora Yanma Wooper Quagsire Espeon Umbreon Murkrow Slowking Misdreavus Unown Wobbuffet Girafarig Pineco Forretress Dunsparce Gligar Steelix Snubbull Granbull Qwilfish Scizor Shuckle Heracross Sneasel Teddiursa Ursaring Slugma Magcargo Swinub Piloswine Corsola Remoraid Octillery Delibird Mantine Skarmory Houndour Houndoom Kingdra Phanpy Donphan Porygon2 Stantler Smeargle Tyrogue Hitmontop Smoochum Elekid Magby Miltank Blissey Raikou Entei Suicune Larvitar Pupitar Tyranitar Lugia Hooh Celebi Treecko Grovyle Sceptile Torchic Combusken Blaziken Mudkip Marshtomp Swampert Ralts Kirlia Gardevoir Aron Lairon Aggron Swablu Altaria Spheal Sealeo Walrein Bagon Shelgon Salamence Beldum Metang Metagross Turtwig Grotle Torterra Chimchar Monferno Infernape Piplup Prinplup Empoleon Shinx Luxio Luxray Riolu Lucario Leafeon Glaceon Sylveon Crobat Poochyena Mightyena Zigzagoon Linoone Wurmple Silcoon Beautifly Cascoon Dustox Lotad Lombre Ludicolo Seedot Nuzleaf Shiftry Taillow Swellow Wingull Pelipper Surskit Masquerain Shroomish Breloom Nincada Ninjask Shedinja Makuhita Hariyama Azurill Nosepass Skitty Delcatty Sableye Mawile Meditite Medicham Electrike Manectric Plusle Minun Volbeat Illumise Roselia Carvanha Sharpedo Numel Camerupt Torkoal Spinda Trapinch Vibrava Flygon Cacnea Cacturne Zangoose Seviper Lunatone Solrock Barboach Whiscash Corphish Crawdaunt Baltoy Claydol Lileep Cradily Anorith Armaldo Milotic Castform Kecleon Shuppet Banette Duskull Dusclops Tropius Chimecho Absol Wynaut Snorunt Glalie Clamperl Huntail Gorebyss Relicanth Luvdisc Regirock Regice Registeel Latias Latios Kyogre Groudon Rayquaza Jirachi Deoxys Starly Staravia Staraptor Bidoof Bibarel Kricketot Kricketune Budew Roserade Cranidos Rampardos Shieldon Bastiodon Burmy Wormadam Mothim Combee Vespiquen Pachirisu Buizel Floatzel Cherubi Cherrim Shellos Gastrodon Ambipom Drifloon Drifblim Buneary Lopunny Mismagius Honchkrow Glameow Chingling Bronzor Bronzong Bonsly Happiny Chatot Spiritomb Gible Gabite Garchomp Hippopotas Hippowdon Skorupi Drapion Croagunk Toxicroak Carnivine Finneon Lumineon Mantyke Snover Abomasnow Weavile Magnezone Rhyperior Tangrowth Electivire Magmortar Togekiss Yanmega Gliscor Mamoswine Gallade Probopass Dusknoir Froslass Rotom Uxie Mesprit Azelf Dialga Palkia Heatran Regigigas Giratina Cresselia Phione Manaphy Darkrai Shaymin Arceus`)

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
	if len(rows) == 0 || !knownStudentHeader(rows[0]) {
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
		if len(row) > 9 {
			extraTime, e = strconv.ParseBool(row[9])
			if e != nil {
				return nil, fmt.Errorf("private student table row %d: invalid extra time flag", i+2)
			}
		}
		source := ""
		if len(row) > 10 {
			if source = row[10]; source != AliasFromCanvasID && source != AliasFromTeacher {
				return nil, fmt.Errorf("private student table row %d: invalid alias source", i+2)
			}
		}
		quizTime, reducedChoices := "", false
		if len(row) > 11 {
			quizTime = row[11]
			if !ValidQuizTime(quizTime) {
				return nil, fmt.Errorf("private student table row %d: invalid quiz_time", i+2)
			}
			reducedChoices, e = strconv.ParseBool(row[12])
			if e != nil {
				return nil, fmt.Errorf("private student table row %d: invalid reduced_choices flag", i+2)
			}
		}
		s.Rows = append(s.Rows, Student{Ref: row[0], Instance: row[1], UserID: id, Name: row[3], FirstName: row[4], Alias: row[5], SISID: row[6], Login: row[7], Monitored: flag, ExtraTime: extraTime, AliasSource: source, QuizTime: quizTime, ReducedChoices: reducedChoices})
	}
	if e = s.validate(); e != nil {
		return nil, e
	}
	if len(rows[0]) < len(aliasSourceStudentHeader) {
		s.renameFromCanvasIDs()
		if e = s.Save(); e != nil {
			return nil, fmt.Errorf("could not save the one-time student alias update: %w", e)
		}
	}
	return s, nil
}
func knownStudentHeader(row []string) bool {
	for _, h := range [][]string{studentHeader, aliasSourceStudentHeader, extraTimeStudentHeader, legacyStudentHeader} {
		if strings.Join(row, "\x00") == strings.Join(h, "\x00") {
			return true
		}
	}
	return false
}

// ValidQuizTime accepts standard, the two supported percentages, or positive
// whole extra minutes within Canvas's maximum. It does not imply application.
func ValidQuizTime(value string) bool {
	if value == "" || value == "50%" || value == "100%" {
		return true
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
	}
	minutes, err := strconv.Atoi(value)
	return err == nil && minutes > 0 && minutes <= 10080
}

// renameFromCanvasIDs is the one-time move away from random aliases. Pool names
// are replaced by Canvas-ID names; any other name was typed by the teacher and stays.
func (s *StudentStore) renameFromCanvasIDs() {
	pool := map[string]bool{}
	for _, a := range aliasPool {
		pool[strings.ToLower(a)] = true
	}
	used := map[string]map[string]bool{}
	var derive []int
	for i, v := range s.Rows {
		if used[v.Instance] == nil {
			used[v.Instance] = map[string]bool{}
		}
		if pool[strings.ToLower(v.Alias)] {
			derive = append(derive, i)
			continue
		}
		s.Rows[i].AliasSource = AliasFromTeacher
		used[v.Instance][strings.ToLower(v.Alias)] = true
	}
	sort.Slice(derive, func(a, b int) bool {
		x, y := s.Rows[derive[a]], s.Rows[derive[b]]
		return x.Instance < y.Instance || x.Instance == y.Instance && x.UserID < y.UserID
	})
	for _, i := range derive {
		v := &s.Rows[i]
		v.Alias, v.AliasSource = idAlias(v.Instance, v.UserID, used[v.Instance]), AliasFromCanvasID
		used[v.Instance][strings.ToLower(v.Alias)] = true
	}
}

// idAlias walks the student's own sequence of pool positions, derived from the
// Canvas instance and user ID, and returns the first unused name. Two installs
// that never shared a table agree on every student except where two students
// wanted the same name and were first seen in a different order.
func idAlias(instance string, userID int64, used map[string]bool) string {
	for k := 0; k < 4*len(aliasPool); k++ {
		sum := sha256.Sum256([]byte(instance + "\x00" + strconv.FormatInt(userID, 10) + "\x00" + strconv.Itoa(k)))
		a := aliasPool[binary.BigEndian.Uint64(sum[:8])%uint64(len(aliasPool))]
		if !used[strings.ToLower(a)] {
			return a
		}
	}
	for _, a := range aliasPool {
		if !used[strings.ToLower(a)] {
			return a
		}
	}
	return ""
}
func (s *StudentStore) validate() error {
	refs, ids, aliases := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i, v := range s.Rows {
		key := v.Instance + "/" + strconv.FormatInt(v.UserID, 10)
		aliasKey := v.Instance + "/" + strings.ToLower(v.Alias)
		if v.Ref == "" || refs[v.Ref] || v.UserID <= 0 || v.Instance == "" || ids[key] || strings.TrimSpace(v.Alias) == "" || aliases[aliasKey] {
			return fmt.Errorf("private student table contains missing or duplicate references, identities, or aliases")
		}
		if strings.ContainsAny(v.Ref, "/\\:\x00") {
			return fmt.Errorf("private student table has invalid stable reference")
		}
		if !ValidQuizTime(v.QuizTime) {
			return fmt.Errorf("private student table row %d: invalid quiz_time", i+2)
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
		_ = w.Write([]string{v.Ref, v.Instance, strconv.FormatInt(v.UserID, 10), v.Name, v.FirstName, v.Alias, v.SISID, v.Login, strconv.FormatBool(v.Monitored), strconv.FormatBool(v.ExtraTime), v.AliasSource, v.QuizTime, strconv.FormatBool(v.ReducedChoices)})
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
	alias := idAlias(instance, userID, used)
	if alias == "" {
		return Student{}, fmt.Errorf("student alias pool exhausted; existing aliases are preserved")
	}
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return Student{}, e
	}
	v := Student{Ref: "s-" + hex.EncodeToString(nonce[:]), Instance: instance, UserID: userID, Name: name, FirstName: firstName(name, shortName), Alias: alias, SISID: sisID, Login: login, AliasSource: AliasFromCanvasID}
	s.Rows = append(s.Rows, v)
	return v, nil
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
