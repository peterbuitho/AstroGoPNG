package lookup

// Built-in knowledge SIMBAD lacks: the Caldwell catalogue (not in SIMBAD at
// all) and popular nicknames it does not carry. Port of the Rust catalog.rs.

type caldwellEntry struct {
	n     int
	desig []string // first = the one to look up in SIMBAD
	name  string   // "" if none
}

var caldwell = []caldwellEntry{
	{1, []string{"NGC 188"}, "Polarissima Cluster"},
	{2, []string{"NGC 40"}, "Bow-Tie Nebula"},
	{3, []string{"NGC 4236"}, ""},
	{4, []string{"NGC 7023"}, "Iris Nebula"},
	{5, []string{"IC 342"}, "Hidden Galaxy"},
	{6, []string{"NGC 6543"}, "Cat's Eye Nebula"},
	{7, []string{"NGC 2403"}, ""},
	{8, []string{"NGC 559"}, ""},
	{9, []string{"Sh2-155"}, "Cave Nebula"},
	{10, []string{"NGC 663"}, ""},
	{11, []string{"NGC 7635"}, "Bubble Nebula"},
	{12, []string{"NGC 6946"}, "Fireworks Galaxy"},
	{13, []string{"NGC 457"}, "Owl Cluster"},
	{14, []string{"NGC 869", "NGC 884"}, "Double Cluster"},
	{15, []string{"NGC 6826"}, "Blinking Planetary"},
	{16, []string{"NGC 7243"}, ""},
	{17, []string{"NGC 147"}, ""},
	{18, []string{"NGC 185"}, ""},
	{19, []string{"IC 5146"}, "Cocoon Nebula"},
	{20, []string{"NGC 7000"}, "North America Nebula"},
	{21, []string{"NGC 4449"}, ""},
	{22, []string{"NGC 7662"}, "Blue Snowball Nebula"},
	{23, []string{"NGC 891"}, "Silver Sliver Galaxy"},
	{24, []string{"NGC 1275"}, "Perseus A"},
	{25, []string{"NGC 2419"}, "Intergalactic Wanderer"},
	{26, []string{"NGC 4244"}, "Silver Needle Galaxy"},
	{27, []string{"NGC 6888"}, "Crescent Nebula"},
	{28, []string{"NGC 752"}, ""},
	{29, []string{"NGC 5005"}, ""},
	{30, []string{"NGC 7331"}, "Deer Lick Group"},
	{31, []string{"IC 405"}, "Flaming Star Nebula"},
	{32, []string{"NGC 4631"}, "Whale Galaxy"},
	{33, []string{"NGC 6992"}, "Eastern Veil Nebula"},
	{34, []string{"NGC 6960"}, "Western Veil Nebula"},
	{35, []string{"NGC 4889"}, ""},
	{36, []string{"NGC 4559"}, ""},
	{37, []string{"NGC 6885"}, ""},
	{38, []string{"NGC 4565"}, "Needle Galaxy"},
	{39, []string{"NGC 2392"}, "Eskimo Nebula"},
	{40, []string{"NGC 3626"}, ""},
	{41, []string{"Mel 25"}, "Hyades"},
	{42, []string{"NGC 7006"}, ""},
	{43, []string{"NGC 7814"}, "Little Sombrero Galaxy"},
	{44, []string{"NGC 7479"}, "Superman Galaxy"},
	{45, []string{"NGC 5248"}, ""},
	{46, []string{"NGC 2261"}, "Hubble's Variable Nebula"},
	{47, []string{"NGC 6934"}, ""},
	{48, []string{"NGC 2775"}, ""},
	{49, []string{"NGC 2237"}, "Rosette Nebula"},
	{50, []string{"NGC 2244"}, ""},
	{51, []string{"IC 1613"}, ""},
	{52, []string{"NGC 4697"}, ""},
	{53, []string{"NGC 3115"}, "Spindle Galaxy"},
	{54, []string{"NGC 2506"}, ""},
	{55, []string{"NGC 7009"}, "Saturn Nebula"},
	{56, []string{"NGC 246"}, "Skull Nebula"},
	{57, []string{"NGC 6822"}, "Barnard's Galaxy"},
	{58, []string{"NGC 2360"}, "Caroline's Cluster"},
	{59, []string{"NGC 3242"}, "Ghost of Jupiter"},
	{60, []string{"NGC 4038"}, "Antennae Galaxies"},
	{61, []string{"NGC 4039"}, "Antennae Galaxies"},
	{62, []string{"NGC 247"}, ""},
	{63, []string{"NGC 7293"}, "Helix Nebula"},
	{64, []string{"NGC 2362"}, "Tau Canis Majoris Cluster"},
	{65, []string{"NGC 253"}, "Sculptor Galaxy"},
	{66, []string{"NGC 5694"}, ""},
	{67, []string{"NGC 1097"}, ""},
	{68, []string{"NGC 6729"}, ""},
	{69, []string{"NGC 6302"}, "Butterfly Nebula"},
	{70, []string{"NGC 300"}, "Sculptor Pinwheel Galaxy"},
	{71, []string{"NGC 2477"}, ""},
	{72, []string{"NGC 55"}, "String of Pearls Galaxy"},
	{73, []string{"NGC 1851"}, ""},
	{74, []string{"NGC 3132"}, "Eight-Burst Nebula"},
	{75, []string{"NGC 6124"}, ""},
	{76, []string{"NGC 6231"}, ""},
	{77, []string{"NGC 5128"}, "Centaurus A"},
	{78, []string{"NGC 6541"}, ""},
	{79, []string{"NGC 3201"}, ""},
	{80, []string{"NGC 5139"}, "Omega Centauri"},
	{81, []string{"NGC 6352"}, ""},
	{82, []string{"NGC 6193"}, ""},
	{83, []string{"NGC 4945"}, ""},
	{84, []string{"NGC 5286"}, ""},
	{85, []string{"IC 2391"}, "Omicron Velorum Cluster"},
	{86, []string{"NGC 6397"}, ""},
	{87, []string{"NGC 1261"}, ""},
	{88, []string{"NGC 5823"}, ""},
	{89, []string{"NGC 6087"}, "S Normae Cluster"},
	{90, []string{"NGC 2867"}, ""},
	{91, []string{"NGC 3532"}, "Wishing Well Cluster"},
	{92, []string{"NGC 3372"}, "Carina Nebula"},
	{93, []string{"NGC 6752"}, "Great Peacock Globular"},
	{94, []string{"NGC 4755"}, "Jewel Box Cluster"},
	{95, []string{"NGC 6025"}, ""},
	{96, []string{"NGC 2516"}, "Southern Beehive Cluster"},
	{97, []string{"NGC 3766"}, "Pearl Cluster"},
	{98, []string{"NGC 4609"}, ""},
	{99, []string{"Coalsack"}, "Coalsack Nebula"},
	{100, []string{"IC 2944"}, "Running Chicken Nebula"},
	{101, []string{"NGC 6744"}, ""},
	{102, []string{"IC 2602"}, "Southern Pleiades"},
	{103, []string{"NGC 2070"}, "Tarantula Nebula"},
	{104, []string{"NGC 362"}, ""},
	{105, []string{"NGC 4833"}, ""},
	{106, []string{"NGC 104"}, "47 Tucanae"},
	{107, []string{"NGC 6101"}, ""},
	{108, []string{"NGC 4372"}, ""},
	{109, []string{"NGC 3195"}, ""},
}

var popularNames = [][2]string{
	{"M 1", "Crab Nebula"}, {"M 6", "Butterfly Cluster"}, {"M 7", "Ptolemy's Cluster"},
	{"M 8", "Lagoon Nebula"}, {"M 11", "Wild Duck Cluster"}, {"M 12", "Gumball Globular"},
	{"M 13", "Great Hercules Cluster"}, {"M 15", "Great Pegasus Cluster"}, {"M 16", "Eagle Nebula"},
	{"M 17", "Omega Nebula"}, {"M 20", "Trifid Nebula"}, {"M 22", "Great Sagittarius Cluster"},
	{"M 24", "Sagittarius Star Cloud"}, {"M 27", "Dumbbell Nebula"}, {"M 29", "Cooling Tower"},
	{"M 30", "Jellyfish Cluster"}, {"M 31", "Andromeda Galaxy"}, {"M 33", "Triangulum Galaxy"},
	{"M 34", "Spiral Cluster"}, {"M 35", "Shoe-Buckle Cluster"}, {"M 36", "Pinwheel Cluster"},
	{"M 38", "Starfish Cluster"}, {"M 40", "Winnecke 4"}, {"M 41", "Little Beehive Cluster"},
	{"M 42", "Orion Nebula"}, {"M 43", "De Mairan's Nebula"}, {"M 44", "Beehive Cluster"},
	{"M 45", "Pleiades"}, {"M 50", "Heart-Shaped Cluster"}, {"M 51", "Whirlpool Galaxy"},
	{"M 52", "Salt and Pepper Cluster"}, {"M 55", "Specter Cluster"}, {"M 57", "Ring Nebula"},
	{"M 61", "Swelling Spiral Galaxy"}, {"M 62", "Flickering Globular Cluster"}, {"M 63", "Sunflower Galaxy"},
	{"M 64", "Black Eye Galaxy"}, {"M 65", "Leo Triplet"}, {"M 66", "Leo Triplet"},
	{"M 67", "Golden Eye Cluster"}, {"M 71", "Angelfish Cluster"}, {"M 74", "Phantom Galaxy"},
	{"M 76", "Little Dumbbell Nebula"}, {"M 77", "Cetus A"}, {"M 78", "Casper the Friendly Ghost Nebula"},
	{"M 81", "Bode's Galaxy"}, {"M 82", "Cigar Galaxy"}, {"M 83", "Southern Pinwheel Galaxy"},
	{"M 87", "Virgo A"}, {"M 93", "Critter Cluster"}, {"M 94", "Croc's Eye Galaxy"},
	{"M 97", "Owl Nebula"}, {"M 99", "Coma Pinwheel Galaxy"}, {"M 101", "Pinwheel Galaxy"},
	{"M 102", "Spindle Galaxy"}, {"M 104", "Sombrero Galaxy"}, {"M 107", "Crucifix Cluster"},
	{"M 108", "Surfboard Galaxy"},
	{"NGC 281", "Pacman Nebula"}, {"NGC 896", "Fish Head Nebula"}, {"NGC 1333", "Embryo Nebula"},
	{"NGC 1360", "Robin's Egg Nebula"}, {"NGC 1435", "Merope Nebula"}, {"NGC 1491", "Fossil Footprint Nebula"},
	{"NGC 1499", "California Nebula"}, {"NGC 1514", "Crystal Ball Nebula"}, {"NGC 1535", "Cleopatra's Eye"},
	{"NGC 1555", "Hind's Variable Nebula"}, {"NGC 1579", "Northern Trifid Nebula"}, {"NGC 1931", "Fly Nebula"},
	{"NGC 1977", "Running Man Nebula"}, {"NGC 2024", "Flame Nebula"}, {"NGC 2170", "Angel Nebula"},
	{"NGC 2174", "Monkey Head Nebula"}, {"NGC 2264", "Christmas Tree Cluster"}, {"NGC 2359", "Thor's Helmet"},
	{"NGC 2371", "Gemini Nebula"}, {"NGC 2467", "Skull and Crossbones Nebula"}, {"NGC 2736", "Pencil Nebula"},
	{"NGC 3324", "Gabriela Mistral Nebula"}, {"NGC 3576", "Statue of Liberty Nebula"}, {"NGC 3918", "Blue Planetary Nebula"},
	{"NGC 5189", "Spiral Planetary Nebula"}, {"NGC 6164", "Dragon's Egg Nebula"}, {"NGC 6188", "Rim Nebula"},
	{"NGC 6210", "Turtle Nebula"}, {"NGC 6334", "Cat's Paw Nebula"}, {"NGC 6357", "War and Peace Nebula"},
	{"NGC 6369", "Little Ghost Nebula"}, {"NGC 6537", "Red Spider Nebula"}, {"NGC 6572", "Blue Racquetball Nebula"},
	{"NGC 6751", "Glowing Eye Nebula"}, {"NGC 6818", "Little Gem Nebula"}, {"NGC 6905", "Blue Flash Nebula"},
	{"NGC 6979", "Pickering's Triangle"}, {"NGC 6995", "Bat Nebula"}, {"NGC 7008", "Fetus Nebula"},
	{"NGC 7027", "Jewel Bug Nebula"}, {"NGC 7380", "Wizard Nebula"}, {"NGC 7822", "Teddy Bear Nebula"},
	{"NGC 7538", "Northern Lagoon Nebula"},
	{"IC 63", "Ghost of Cassiopeia"}, {"IC 410", "Tadpoles Nebula"}, {"IC 417", "Spider Nebula"},
	{"IC 443", "Jellyfish Nebula"}, {"IC 1318", "Sadr Region"}, {"IC 1396", "Elephant's Trunk Nebula"},
	{"IC 1795", "Fish Head Nebula"}, {"IC 1805", "Heart Nebula"}, {"IC 1848", "Soul Nebula"},
	{"IC 2118", "Witch Head Nebula"}, {"IC 2177", "Seagull Nebula"}, {"IC 4406", "Retina Nebula"},
	{"IC 4592", "Blue Horsehead Nebula"}, {"IC 4604", "Rho Ophiuchi Nebula"}, {"IC 4628", "Prawn Nebula"},
	{"IC 5070", "Pelican Nebula"},
	{"Sh2-82", "Little Cocoon Nebula"}, {"Sh2-101", "Tulip Nebula"}, {"Sh2-106", "Celestial Snow Angel"},
	{"Sh2-114", "Flying Dragon Nebula"}, {"Sh2-129", "Flying Bat Nebula"}, {"Sh2-132", "Lion Nebula"},
	{"Sh2-142", "Wizard Nebula"}, {"Sh2-157", "Lobster Claw Nebula"}, {"Sh2-158", "Northern Lagoon Nebula"},
	{"Sh2-162", "Bubble Nebula"}, {"Sh2-190", "Heart Nebula"}, {"Sh2-199", "Soul Nebula"},
	{"Sh2-206", "Fossil Footprint Nebula"}, {"Sh2-220", "California Nebula"}, {"Sh2-229", "Flaming Star Nebula"},
	{"Sh2-236", "Tadpoles Nebula"}, {"Sh2-240", "Spaghetti Nebula"}, {"Sh2-248", "Jellyfish Nebula"},
	{"Sh2-252", "Monkey Head Nebula"}, {"Sh2-261", "Lower's Nebula"}, {"Sh2-264", "Angelfish Nebula"},
	{"Sh2-273", "Cone Nebula"}, {"Sh2-274", "Medusa Nebula"}, {"Sh2-275", "Rosette Nebula"},
	{"Sh2-276", "Barnard's Loop"}, {"Sh2-279", "Running Man Nebula"}, {"Sh2-296", "Seagull Nebula"},
	{"Sh2-308", "Dolphin Head Nebula"},
	{"Barnard 33", "Horsehead Nebula"}, {"Barnard 72", "Snake Nebula"}, {"Barnard 150", "Seahorse Nebula"},
	{"LDN 1235", "Dark Shark Nebula"}, {"LDN 1622", "Boogeyman Nebula"}, {"vdB 141", "Ghost Nebula"},
	{"NGC 1316", "Fornax A"}, {"NGC 1317", "Fornax B"}, {"NGC 1365", "Great Barred Spiral Galaxy"},
	{"NGC 1566", "Spanish Dancer Galaxy"}, {"NGC 2442", "Meathook Galaxy"}, {"NGC 2537", "Bear's Paw Galaxy"},
	{"NGC 2683", "UFO Galaxy"}, {"NGC 2841", "Tiger's Eye Galaxy"}, {"NGC 3184", "Little Pinwheel Galaxy"},
	{"NGC 3344", "Sliced Onion Galaxy"}, {"NGC 3521", "Bubble Galaxy"}, {"NGC 3628", "Hamburger Galaxy"},
	{"NGC 4435", "Eyes Galaxies"}, {"NGC 4438", "Eyes Galaxies"}, {"NGC 4490", "Cocoon Galaxy"},
	{"NGC 4535", "Lost Galaxy"}, {"NGC 4567", "Butterfly Galaxies"}, {"NGC 4568", "Siamese Twins"},
	{"NGC 4656", "Hockey Stick Galaxy"}, {"NGC 4676", "Mice Galaxies"}, {"NGC 5907", "Splinter Galaxy"},
	{"NGC 6503", "Lost-in-Space Galaxy"}, {"IC 2574", "Coddington's Nebula"}, {"UGC 10214", "Tadpole Galaxy"},
	{"NGC 2169", "37 Cluster"}, {"NGC 3293", "Gem Cluster"}, {"NGC 6811", "Hole in a Cluster"},
	{"NGC 6819", "Foxhead Cluster"}, {"NGC 6939", "Ghost Bush Cluster"}, {"NGC 7789", "Caroline's Rose"},
	{"Mel 20", "Alpha Persei Cluster"}, {"Mel 111", "Coma Star Cluster"}, {"Cr 399", "Coathanger"},
}

func caldwellNumber(designations []string) (int, bool) {
	for _, c := range caldwell {
		for _, cd := range c.desig {
			for _, d := range designations {
				if normEq(d, cd) {
					return c.n, true
				}
			}
		}
	}
	return 0, false
}

func caldwellTarget(query string) (string, bool) {
	nq := normalize(query)
	if len(nq) < 2 || nq[0] != 'C' {
		return "", false
	}
	n, ok := atoi(nq[1:])
	if !ok {
		return "", false
	}
	for _, c := range caldwell {
		if c.n == n {
			return c.desig[0], true
		}
	}
	return "", false
}

func popularName(designations []string) (string, bool) {
	for _, c := range caldwell {
		if c.name == "" {
			continue
		}
		for _, cd := range c.desig {
			for _, d := range designations {
				if normEq(d, cd) {
					return c.name, true
				}
			}
		}
	}
	for _, p := range popularNames {
		for _, d := range designations {
			if normEq(d, p[0]) {
				return p[1], true
			}
		}
	}
	return "", false
}
