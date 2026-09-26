package games

import (
	"fmt"
	s "strings"
)

const (
	CategoryArcade               = "Arcade"
	CategoryComputer             = "Computer"
	CategoryConsole              = "Console"
	CategoryHandheld             = "Handheld"
	CategoryOther                = "Other"
	ManufacturerAcorn            = "Acorn"
	ManufacturerAmstrad          = "Amstrad"
	ManufacturerApogee           = "Apogee"
	ManufacturerApple            = "Apple"
	ManufacturerAtari            = "Atari"
	ManufacturerBally            = "Bally"
	ManufacturerBandai           = "Bandai"
	ManufacturerBenesse          = "Benesse"
	ManufacturerBitCorp          = "Bit Corporation"
	ManufacturerCambridge        = "Cambridge"
	ManufacturerCasio            = "Casio"
	ManufacturerColeco           = "Coleco"
	ManufacturerCommodore        = "Commodore"
	ManufacturerCompukit         = "Compukit"
	ManufacturerDEC              = "DEC"
	ManufacturerElektronika      = "Elektronika"
	ManufacturerEmerson          = "Emerson"
	ManufacturerEntex            = "Entex"
	ManufacturerFairchild        = "Fairchild"
	ManufacturerGCE              = "GCE"
	ManufacturerIBM              = "IBM"
	ManufacturerInteract         = "Interact"
	ManufacturerInterton         = "Interton"
	ManufacturerJupiter          = "Jupiter"
	ManufacturerMagnavox         = "Magnavox"
	ManufacturerMattel           = "Mattel"
	ManufacturerMicrosoft        = "Microsoft"
	ManufacturerMiSTer           = "MiSTer"
	ManufacturerMilesGordon      = "Miles Gordon Technology"
	ManufacturerNEC              = "NEC"
	ManufacturerNintendo         = "Nintendo"
	ManufacturerPanasonic        = "Panasonic"
	ManufacturerPEL              = "PEL Varazdin"
	ManufacturerPhilips          = "Philips"
	ManufacturerSega             = "Sega"
	ManufacturerSharp            = "Sharp"
	ManufacturerSinclair         = "Sinclair"
	ManufacturerSNK              = "SNK"
	ManufacturerSony             = "Sony"
	ManufacturerSord             = "Sord"
	ManufacturerSpecNext         = "SpecNext"
	ManufacturerSpectravideo     = "Spectravideo"
	ManufacturerTandy            = "Tandy"
	ManufacturerTangerine        = "Tangerine"
	ManufacturerTatung           = "Tatung"
	ManufacturerTesla            = "Tesla"
	ManufacturerTexasInstruments = "Texas Instruments"
	ManufacturerTomy             = "Tomy"
	ManufacturerVideoTechnology  = "Video Technology"
	ManufacturerVTech            = "VTech"
	ManufacturerWatara           = "Watara"
)

type MglParams struct {
	Delay  int
	Method string
	Index  int
}

type Slot struct {
	Exts []string
	Mgl  *MglParams
}

type System struct {
	Id             string
	Name           string // US
	Category       string
	ReleaseDate    string // US
	Manufacturer   string
	Alias          []string
	SetName        string
	SetNameSameDir bool
	Folder         []string
	Rbf            string
	Slots          []Slot
}

func PathToMglDef(system System, path string) (*MglParams, error) {
	var mglDef *MglParams

	for _, ft := range system.Slots {
		for _, ext := range ft.Exts {
			if s.HasSuffix(s.ToLower(path), ext) {
				return ft.Mgl, nil
			}
		}
	}

	return mglDef, fmt.Errorf("system has no matching mgl args: %s, %s", system.Id, path)
}

var Systems = map[string]System{
	// Consoles
	"3DO": {
		Id:           "3DO",
		Name:         "3DO",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerPanasonic,
		ReleaseDate:  "1993-04-10",
		Folder:       []string{"3DO"},
		Alias:        []string{"3DO"},
		Rbf:          "_Console/3DO",
		Slots: []Slot{
			{
				Exts: []string{".cue", ".chd", ".iso"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"AdventureVision": {
		Id:           "AdventureVision",
		Name:         "Adventure Vision",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerEntex,
		ReleaseDate:  "1982-08-01",
		Alias:        []string{"AVision"},
		Folder:       []string{"AVision"},
		Rbf:          "_Console/AdventureVision",
		Slots: []Slot{
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"Arcadia": {
		Id:           "Arcadia",
		Name:         "Arcadia 2001",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerEmerson,
		ReleaseDate:  "1982-05-01",
		Folder:       []string{"Arcadia"},
		Rbf:          "_Console/Arcadia",
		Slots: []Slot{
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Astrocade": {
		Id:           "Astrocade",
		Name:         "Bally Astrocade",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerBally,
		ReleaseDate:  "1978-04-01",
		Folder:       []string{"Astrocade"},
		Rbf:          "_Console/Astrocade",
		Slots: []Slot{
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Atari2600": {
		Id:           "Atari2600",
		Name:         "Atari 2600",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerAtari,
		ReleaseDate:  "1977-09-11",
		Folder:       []string{"ATARI7800", "Atari2600"},
		SetName:      "Atari2600",
		Rbf:          "_Console/Atari7800",
		Slots: []Slot{
			{
				Exts: []string{".a26"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"Atari5200": {
		Id:           "Atari5200",
		Name:         "Atari 5200",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerAtari,
		ReleaseDate:  "1982-11-01",
		Folder:       []string{"ATARI5200"},
		Rbf:          "_Console/Atari5200",
		Slots: []Slot{
			{
				Exts: []string{".car", ".a52", ".bin", ".rom"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"Atari7800": {
		Id:           "Atari7800",
		Name:         "Atari 7800",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerAtari,
		ReleaseDate:  "1986-05-01",
		Folder:       []string{"ATARI7800"},
		Rbf:          "_Console/Atari7800",
		Slots: []Slot{
			{
				Exts: []string{".a78", ".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"AtariLynx": {
		Id:           "AtariLynx",
		Name:         "Atari Lynx",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerAtari,
		ReleaseDate:  "1989-09-01",
		Folder:       []string{"AtariLynx"},
		Rbf:          "_Console/AtariLynx",
		Slots: []Slot{
			{
				Exts: []string{".lnx"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"CasioPV1000": {
		Id:           "CasioPV1000",
		Name:         "Casio PV-1000",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerCasio,
		ReleaseDate:  "1983-10-01",
		Alias:        []string{"Casio_PV-1000"},
		Folder:       []string{"Casio_PV-1000"},
		Rbf:          "_Console/Casio_PV-1000",
		Slots: []Slot{
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"CDI": {
		Id:           "CDI",
		Name:         "CD-i",
		Category:     CategoryConsole,
		ReleaseDate:  "1991-12-03",
		Manufacturer: ManufacturerPhilips,
		Folder:       []string{"CD-i"},
		Rbf:          "_Console/CDi",
		Slots: []Slot{
			{
				Exts: []string{".cue", ".chd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"ChannelF": {
		Id:           "ChannelF",
		Name:         "Channel F",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerFairchild,
		ReleaseDate:  "1976-11-01",
		Folder:       []string{"ChannelF"},
		Rbf:          "_Console/ChannelF",
		Slots: []Slot{
			{
				Exts: []string{".rom", ".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"ColecoVision": {
		Id:           "ColecoVision",
		Name:         "ColecoVision",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerColeco,
		ReleaseDate:  "1982-08-01",
		Alias:        []string{"Coleco"},
		Folder:       []string{"Coleco"},
		Rbf:          "_Console/ColecoVision",
		Slots: []Slot{
			{
				// Index 1 on purpose: the core has two index-0 entries (carts, then
				// SG-1000) and MiSTer uses the last match, so f0 would load carts as
				// SG-1000. No index-1 entry means MiSTer falls back to the first menu
				// line, which is the cart loader.
				Exts: []string{".col", ".bin", ".rom"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"CreatiVision": {
		Id:           "CreatiVision",
		Name:         "VTech CreatiVision",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerVTech,
		ReleaseDate:  "1981-01-01",
		Folder:       []string{"CreatiVision"},
		Rbf:          "_Console/CreatiVision",
		Slots: []Slot{
			{
				Exts: []string{".rom", ".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
			{
				Exts: []string{".bas"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  3,
				},
			},
		},
	},
	"FDS": {
		Id:           "FDS",
		Name:         "Famicom Disk System",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "1986-02-21",
		SetName:      "FDS",
		Alias:        []string{"FamicomDiskSystem"},
		Folder:       []string{"NES", "FDS"},
		Rbf:          "_Console/NES",
		Slots: []Slot{
			{
				Exts: []string{".fds"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Gamate": {
		Id:           "Gamate",
		Name:         "Gamate",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerBitCorp,
		ReleaseDate:  "1990-01-01",
		Folder:       []string{"Gamate"},
		Rbf:          "_Console/Gamate",
		Slots: []Slot{
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"GameAndWatch": {
		Id:           "GameAndWatch",
		Name:         "Game & Watch",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "1980-04-28",
		Folder:       []string{"Game and Watch"},
		Rbf:          "_Console/GameAndWatch",
		Slots: []Slot{
			{
				Exts: []string{".gnw"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Gameboy": {
		Id:           "Gameboy",
		Name:         "Gameboy",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "1989-04-21",
		Alias:        []string{"GB"},
		Folder:       []string{"GAMEBOY"},
		Rbf:          "_Console/Gameboy",
		Slots: []Slot{
			{
				Exts: []string{".gb"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"Gameboy2P": {
		Id:           "Gameboy2P",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "1989-04-21",
		Name:         "Gameboy (2 Player)",
		Folder:       []string{"GAMEBOY2P"},
		Rbf:          "_Console/Gameboy2P",
		Slots: []Slot{
			{
				Exts: []string{".gb", ".gbc"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"GameboyColor": {
		Id:           "GameboyColor",
		Name:         "Gameboy Color",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "1998-10-21",
		Alias:        []string{"GBC"},
		Folder:       []string{"GAMEBOY", "GBC"},
		SetName:      "GBC",
		Rbf:          "_Console/Gameboy",
		Slots: []Slot{
			{
				Exts: []string{".gbc"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"GameGear": {
		Id:           "GameGear",
		Name:         "Game Gear",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerSega,
		ReleaseDate:  "1990-10-06",
		Alias:        []string{"GG"},
		Folder:       []string{"SMS", "GameGear"},
		SetName:      "GameGear",
		Rbf:          "_Console/SMS",
		Slots: []Slot{
			{
				Exts: []string{".gg"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  2,
				},
			},
		},
	},
	"GBA": {
		Id:           "GBA",
		Name:         "Gameboy Advance",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "2001-03-21",
		Alias:        []string{"GameboyAdvance"},
		Folder:       []string{"GBA"},
		Rbf:          "_Console/GBA",
		Slots: []Slot{
			{
				Exts: []string{".gba"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"GBA2P": {
		Id:           "GBA2P",
		Name:         "Gameboy Advance (2 Player)",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "2001-03-21",
		Folder:       []string{"GBA2P"},
		Rbf:          "_Console/GBA2P",
		Slots: []Slot{
			{
				Exts: []string{".gba"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Intellivision": {
		Id:           "Intellivision",
		Name:         "Intellivision",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerMattel,
		ReleaseDate:  "1979-12-03",
		Folder:       []string{"Intellivision"},
		Rbf:          "_Console/Intellivision",
		Slots: []Slot{
			{
				Exts: []string{".int", ".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"Jaguar": {
		Id:           "Jaguar",
		Name:         "Jaguar",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerAtari,
		ReleaseDate:  "1993-11-23",
		Folder:       []string{"Jaguar"},
		Rbf:          "_Console/Jaguar",
		Slots: []Slot{
			{
				Exts: []string{".jag", ".j64", ".rom", ".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"JaguarCD": {
		Id:           "JaguarCD",
		Name:         "Jaguar CD",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerAtari,
		ReleaseDate:  "1995-09-21",
		Folder:       []string{"Jaguar"},
		Rbf:          "_Console/Jaguar",
		Slots: []Slot{
			{
				Exts: []string{".cdi"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
		},
	},
	"MasterSystem": {
		Id:           "MasterSystem",
		Name:         "Master System",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerSega,
		ReleaseDate:  "1985-10-20",
		Alias:        []string{"SMS"},
		Folder:       []string{"SMS"},
		Rbf:          "_Console/SMS",
		Slots: []Slot{
			{
				Exts: []string{".sms"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"MegaCD": {
		Id:           "MegaCD",
		Name:         "Sega CD",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerSega,
		ReleaseDate:  "1991-12-12",
		Alias:        []string{"SegaCD"},
		Folder:       []string{"MegaCD"},
		Rbf:          "_Console/MegaCD",
		Slots: []Slot{
			{
				Exts: []string{".cue", ".chd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"MegaDrive": {
		Id:           "MegaDrive",
		Name:         "MegaDrive",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerSega,
		ReleaseDate:  "1988-10-29",
		Alias:        []string{"MegaDrive"},
		Folder:       []string{"MegaDrive", "Genesis"},
		Rbf:          "_Console/MegaDrive",
		Slots: []Slot{
			{
				Exts: []string{".bin", ".gen", ".md"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"MegaDuck": {
		Id:           "MegaDuck",
		Name:         "Mega Duck",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerWatara,
		ReleaseDate:  "1993-01-01",
		Folder:       []string{"GAMEBOY", "MegaDuck"},
		Rbf:          "_Console/Gameboy",
		Slots: []Slot{
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"NeoGeo": {
		Id:           "NeoGeo",
		Name:         "Neo-Geo MVS AES",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerSNK,
		ReleaseDate:  "1990-08-22",
		Folder:       []string{"NEOGEO"},
		Rbf:          "_Console/NeoGeo",
		Slots: []Slot{
			{
				Exts: []string{".neo"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"NeoGeoCD": {
		Id:           "NeoGeoCD",
		Name:         "Neo-Geo CD",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerSNK,
		ReleaseDate:  "1994-09-09",
		Folder:       []string{"NeoGeo-CD"},
		Rbf:          "_Console/NeoGeo",
		Slots: []Slot{
			{
				Exts: []string{".cue", ".chd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
		},
	},
	"NeoGeoPocket": {
		Id:           "NeoGeoPocket",
		Name:         "Neo-Geo Pocket",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerSNK,
		ReleaseDate:  "1998-10-28",
		Folder:       []string{"NGPC"},
		Rbf:          "_Console/NGPC",
		Slots: []Slot{
			{
				Exts: []string{".ngp"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"NeoGeoPocket-Color": {
		Id:           "NeoGeoPocket-Color",
		Name:         "Neo-Geo Pocket Color",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerSNK,
		ReleaseDate:  "1999-08-06",
		Folder:       []string{"NGPC"},
		Rbf:          "_Console/NGPC",
		Slots: []Slot{
			{
				Exts: []string{".ngc"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"NES": {
		Id:           "NES",
		Name:         "NES",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "1985-10-18",
		Folder:       []string{"NES"},
		Rbf:          "_Console/NES",
		Slots: []Slot{
			{
				Exts: []string{".nes"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"NESMusic": {
		Id:       "NESMusic",
		Name:     "NES Music",
		Category: CategoryOther,
		Folder:   []string{"NES"},
		Rbf:      "_Console/NES",
		Slots: []Slot{
			{
				Exts: []string{".nsf"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Nintendo64": {
		Id:           "Nintendo64",
		Name:         "Nintendo 64",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "1996-06-23",
		Alias:        []string{"N64"},
		Folder:       []string{"N64"},
		Rbf:          "_Console/N64",
		Slots: []Slot{
			{
				Exts: []string{".n64", ".z64"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"Odyssey2": {
		Id:           "Odyssey2",
		Name:         "Magnavox Odyssey2",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerMagnavox,
		ReleaseDate:  "1978-09-01",
		Folder:       []string{"ODYSSEY2"},
		Rbf:          "_Console/Odyssey2",
		Slots: []Slot{
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"PocketChallengeV2": {
		Id:           "PocketChallengeV2",
		Name:         "Pocket Challenge V2",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerBenesse,
		ReleaseDate:  "2000-01-01",
		Folder:       []string{"WonderSwan", "PocketChallengeV2"},
		SetName:      "PocketChallengeV2",
		Rbf:          "_Console/WonderSwan",
		Slots: []Slot{
			{
				Exts: []string{".pc2"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"PokemonMini": {
		Id:           "PokemonMini",
		Name:         "Pokemon Mini",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "2001-11-16",
		Folder:       []string{"PokemonMini"},
		Rbf:          "_Console/PokemonMini",
		Slots: []Slot{
			{
				Exts: []string{".min"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"PSX": {
		Id:           "PSX",
		Name:         "Playstation",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerSony,
		ReleaseDate:  "1994-12-03",
		Alias:        []string{"Playstation", "PS1"},
		Folder:       []string{"PSX"},
		Rbf:          "_Console/PSX",
		Slots: []Slot{
			{
				Exts: []string{".cue", ".chd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
			{
				Exts: []string{".exe"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"Saturn": {
		Id:           "Saturn",
		Name:         "Saturn",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerSega,
		ReleaseDate:  "1994-11-22",
		Folder:       []string{"Saturn"},
		Rbf:          "_Console/Saturn",
		Slots: []Slot{
			{
				Exts: []string{".cue", ".chd"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"Sega32X": {
		Id:           "Sega32X",
		Name:         "Genesis 32X",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerSega,
		ReleaseDate:  "1994-11-21",
		Alias:        []string{"S32X", "32X"},
		Folder:       []string{"S32X"},
		Rbf:          "_Console/S32X",
		Slots: []Slot{
			{
				Exts: []string{".32x"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"SG1000": {
		Id:           "SG1000",
		Name:         "SG-1000",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerSega,
		ReleaseDate:  "1983-07-15",
		SetName:      "SG1000",
		Folder:       []string{"SG1000", "Coleco", "SMS"},
		Rbf:          "_Console/SMS",
		Slots: []Slot{
			{
				Exts: []string{".sg"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"SNES": {
		Id:           "SNES",
		Name:         "SNES",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "1990-11-21",
		Alias:        []string{"SuperNintendo"},
		Folder:       []string{"SNES"},
		Rbf:          "_Console/SNES",
		Slots: []Slot{
			{
				Exts: []string{".sfc", ".smc", ".bin", ".bs"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"SNESMusic": {
		Id:       "SNESMusic",
		Name:     "SNES Music",
		Category: CategoryOther,
		Folder:   []string{"SNES"},
		Rbf:      "_Console/SNES",
		Slots: []Slot{
			{
				Exts: []string{".spc"},
				Mgl: &MglParams{
					Delay:  2,
					Method: "f",
					Index:  4,
				},
			},
		},
	},
	"SuperGameboy": {
		Id:           "SuperGameboy",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "1994-06-14",
		Name:         "Super Gameboy",
		Alias:        []string{"SGB"},
		Folder:       []string{"SGB"},
		Rbf:          "_Console/SGB",
		Slots: []Slot{
			{
				Exts: []string{".gb", ".gbc"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"SuperGrafx": {
		Id:           "SuperGrafx",
		Name:         "SuperGrafx",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerNEC,
		ReleaseDate:  "1989-12-08",
		Folder:       []string{"TGFX16"},
		Rbf:          "_Console/TurboGrafx16",
		Slots: []Slot{
			{
				Exts: []string{".sgx"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"SuperVision": {
		Id:           "SuperVision",
		Name:         "SuperVision",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerWatara,
		ReleaseDate:  "1992-01-01",
		Folder:       []string{"SuperVision"},
		Rbf:          "_Console/SuperVision",
		Slots: []Slot{
			{
				Exts: []string{".bin", ".sv"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"TurboGrafx16": {
		Id:           "TurboGrafx16",
		Name:         "TurboGrafx-16",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerNEC,
		ReleaseDate:  "1987-10-30",
		Alias:        []string{"TGFX16", "PCEngine"},
		Folder:       []string{"TGFX16"},
		Rbf:          "_Console/TurboGrafx16",
		Slots: []Slot{
			{
				Exts: []string{".bin", ".pce"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"TurboGrafx16CD": {
		Id:           "TurboGrafx16CD",
		Name:         "TurboGrafx-16 CD",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerNEC,
		ReleaseDate:  "1989-11-01",
		Alias:        []string{"TGFX16-CD", "PCEngineCD"},
		Folder:       []string{"TGFX16-CD"},
		Rbf:          "_Console/TurboGrafx16",
		Slots: []Slot{
			{
				Exts: []string{".cue", ".chd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"VC4000": {
		Id:           "VC4000",
		Name:         "VC4000",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerInterton,
		ReleaseDate:  "1978-01-01",
		Folder:       []string{"VC4000"},
		Rbf:          "_Console/VC4000",
		Slots: []Slot{
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Vectrex": {
		Id:           "Vectrex",
		Name:         "Vectrex",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerGCE,
		ReleaseDate:  "1982-11-01",
		Folder:       []string{"VECTREX"},
		Rbf:          "_Console/Vectrex",
		Slots: []Slot{
			{
				Exts: []string{".vec", ".bin", ".rom"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"VirtualBoy": {
		Id:           "VirtualBoy",
		Name:         "Virtual Boy",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerNintendo,
		ReleaseDate:  "1995-08-14",
		Folder:       []string{"VirtualBoy"},
		Rbf:          "_Console/VirtualBoy",
		Slots: []Slot{
			{
				Exts: []string{".vb"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"WonderSwan": {
		Id:           "WonderSwan",
		Name:         "WonderSwan",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerBandai,
		ReleaseDate:  "1999-03-04",
		Folder:       []string{"WonderSwan"},
		Rbf:          "_Console/WonderSwan",
		Slots: []Slot{
			{
				Exts: []string{".ws"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"WonderSwanColor": {
		Id:           "WonderSwanColor",
		Name:         "WonderSwan Color",
		Category:     CategoryHandheld,
		Manufacturer: ManufacturerBandai,
		ReleaseDate:  "1999-12-30",
		Folder:       []string{"WonderSwan", "WonderSwanColor"},
		SetName:      "WonderSwanColor",
		Rbf:          "_Console/WonderSwan",
		Slots: []Slot{
			{
				Exts: []string{".wsc"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	// TODO: AY-3-8500
	//       Doesn't appear to have roms even though it has a folder.
	// TODO: C2650
	//       Not in official repos, think it comes with update_all.
	//       https://github.com/Grabulosaure/C2650_MiSTer
	// TODO: EpochGalaxy2
	//       Has a folder and mount entry but commented as "remove".

	// Computers
	"AcornAtom": {
		Id:           "AcornAtom",
		Name:         "Atom",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerAcorn,
		ReleaseDate:  "1980-03-01",
		Folder:       []string{"AcornAtom"},
		Rbf:          "_Computer/AcornAtom",
		Slots: []Slot{
			{
				Exts: []string{".vhd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"AcornElectron": {
		Id:           "AcornElectron",
		Name:         "Electron",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerAcorn,
		ReleaseDate:  "1983-08-01",
		Folder:       []string{"AcornElectron"},
		Rbf:          "_Computer/AcornElectron",
		Slots: []Slot{
			{
				Exts: []string{".vhd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"AliceMC10": {
		Id:           "AliceMC10",
		Name:         "Tandy MC-10",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerTandy,
		ReleaseDate:  "1983-01-01",
		Folder:       []string{"AliceMC10"},
		Rbf:          "_Computer/AliceMC10",
		Slots: []Slot{
			{
				Exts: []string{".c10"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"Amiga": {
		Id:           "Amiga",
		Name:         "Amiga",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerCommodore,
		ReleaseDate:  "1985-07-23",
		Folder:       []string{"Amiga"},
		Alias:        []string{"Minimig"},
		Rbf:          "_Computer/Minimig",
		Slots: []Slot{
			{
				Exts: []string{".adf"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"AmigaCD32": {
		Id:           "AmigaCD32",
		Name:         "Amiga CD32",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerCommodore,
		ReleaseDate:  "1993-09-17",
		Folder:       []string{"AmigaCD32", "Amiga/CD32"},
		Alias:        []string{"AmigaCD32"},
		Rbf:          "_Computer/Minimig",
		Slots: []Slot{
			{
				Exts: []string{".cue", ".chd", ".iso"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  4,
				},
			},
		},
	},
	"AmigaCDTV": {
		Id:           "AmigaCDTV",
		Name:         "Amiga CDTV",
		Category:     CategoryConsole,
		Manufacturer: ManufacturerCommodore,
		ReleaseDate:  "1991-03-01",
		Folder:       []string{"AmigaCDTV", "Amiga/CDTV"},
		Alias:        []string{"AmigaCDTV"},
		Rbf:          "_Computer/Minimig",
		Slots: []Slot{
			{
				Exts: []string{".cue", ".chd", ".iso"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  5,
				},
			},
		},
	},
	"AmigaVision": {
		Id:           "AmigaVision",
		Name:         "AmigaVision",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerCommodore,
		ReleaseDate:  "1985-07-23",
		Folder:       []string{"Amiga"},
		Alias:        []string{"AmigaVision"},
		Rbf:          "_Computer/Minimig",
		Slots: []Slot{
			{
				Exts: []string{".txt", ".ags"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Amstrad": {
		Id:           "Amstrad",
		Name:         "Amstrad CPC",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerAmstrad,
		ReleaseDate:  "1984-06-21",
		Folder:       []string{"Amstrad"},
		Rbf:          "_Computer/Amstrad",
		Slots: []Slot{
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
			{
				Exts: []string{".e??"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  3,
				},
			},
			{
				Exts: []string{".cdt"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  4,
				},
			},
		},
	},
	"AmstradPCW": {
		Id:           "AmstradPCW",
		Name:         "Amstrad PCW",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerAmstrad,
		ReleaseDate:  "1985-09-01",
		Alias:        []string{"Amstrad-PCW"},
		Folder:       []string{"Amstrad PCW"},
		Rbf:          "_Computer/Amstrad-PCW",
		Slots: []Slot{
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
		},
	},
	"ao486": {
		Id:           "ao486",
		Name:         "PC (486SX)",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerIBM,
		ReleaseDate:  "1989-04-10",
		Folder:       []string{"AO486", "_Computer/_DOS Games"},
		Rbf:          "_Computer/ao486",
		Slots: []Slot{
			{
				Exts: []string{".img", ".ima", ".vfd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".vhd", ".mgl"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  2,
				},
			},
		},
	},
	"Apogee": {
		Id:           "Apogee",
		Name:         "Apogee BK-01",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerApogee,
		ReleaseDate:  "1992-01-01",
		Folder:       []string{"APOGEE"},
		Rbf:          "_Computer/Apogee",
		Slots: []Slot{
			{
				Exts: []string{".rka", ".rkr", ".gam"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"AppleI": {
		Id:           "AppleI",
		Name:         "Apple I",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerApple,
		ReleaseDate:  "1976-04-01",
		Alias:        []string{"Apple-I"},
		Folder:       []string{"Apple-I"},
		Rbf:          "_Computer/Apple-I",
		Slots: []Slot{
			{
				Exts: []string{".txt"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"AppleII": {
		Id:           "AppleII",
		Name:         "Apple IIe",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerApple,
		ReleaseDate:  "1983-01-19",
		Alias:        []string{"Apple-II"},
		Folder:       []string{"Apple-II"},
		Rbf:          "_Computer/Apple-II",
		Slots: []Slot{
			{
				Exts: []string{".nib", ".dsk", ".do", ".po"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".hdv"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
		},
	},
	"Aquarius": {
		Id:           "Aquarius",
		Name:         "Mattel Aquarius",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerMattel,
		ReleaseDate:  "1983-06-01",
		Folder:       []string{"AQUARIUS"},
		Rbf:          "_Computer/Aquarius",
		Slots: []Slot{
			{
				// Index 1 on purpose: carts and tapes are both index 0 in the core
				// and MiSTer uses the last match (tapes). With no index-1 entry MiSTer
				// falls back to the first menu line, the cart loader.
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
			{
				Exts: []string{".caq"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Atari800": {
		Id:           "Atari800",
		Name:         "Atari 800XL",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerAtari,
		ReleaseDate:  "1983-11-01",
		Folder:       []string{"ATARI800"},
		Rbf:          "_Computer/Atari800",
		Slots: []Slot{
			{
				Exts: []string{".atr", ".atx"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  6,
				},
			},
			{
				Exts: []string{".xex"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  5,
				},
			},
			{
				// Mount only: the core's Boot D1 entry lists XDF instead of XFD, so
				// an .xfd sent there would be misread as an ATR image.
				Exts: []string{".xfd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".car", ".rom", ".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  8,
				},
			},
		},
	},
	"BBCMicro": {
		Id:           "BBCMicro",
		Name:         "BBC Micro Master",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerAcorn,
		ReleaseDate:  "1981-12-01",
		Folder:       []string{"BBCMicro"},
		Rbf:          "_Computer/BBCMicro",
		Slots: []Slot{
			{
				Exts: []string{".vhd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".ssd", ".dsd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
			{
				Exts: []string{".ssd", ".dsd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  2,
				},
			},
		},
	},
	"BK0011M": {
		Id:           "BK0011M",
		Name:         "BK0011M",
		Category:     CategoryComputer,
		ReleaseDate:  "1990-01-01",
		Manufacturer: ManufacturerElektronika,
		Folder:       []string{"BK0011M"},
		Rbf:          "_Computer/BK0011M",
		Slots: []Slot{
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
			{
				Exts: []string{".vhd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"C16": {
		Id:           "C16",
		Name:         "Commodore 16",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerCommodore,
		ReleaseDate:  "1984-07-01",
		Folder:       []string{"C16"},
		Rbf:          "_Computer/C16",
		Slots: []Slot{
			{
				Exts: []string{".d64", ".g64"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".d64", ".g64"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
			{
				Exts: []string{".prg", ".tap", ".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"C64": {
		Id:           "C64",
		Name:         "Commodore 64",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerCommodore,
		ReleaseDate:  "1982-08-01",
		Folder:       []string{"C64"},
		Rbf:          "_Computer/C64",
		Slots: []Slot{
			{
				Exts: []string{".d64", ".g64", ".t64", ".d81"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".d64", ".g64", ".t64", ".d81"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
			{
				Exts: []string{".prg", ".crt", ".reu", ".tap"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"CasioPV2000": {
		Id:           "CasioPV2000",
		Name:         "Casio PV-2000",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerCasio,
		ReleaseDate:  "1983-01-01",
		Alias:        []string{"Casio_PV-2000"},
		Folder:       []string{"Casio_PV-2000"},
		Rbf:          "_Computer/Casio_PV-2000",
		Slots: []Slot{
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"CoCo2": {
		Id:           "CoCo2",
		Name:         "TRS-80 CoCo 2",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerTandy,
		ReleaseDate:  "1983-01-01",
		Folder:       []string{"CoCo2"},
		Rbf:          "_Computer/CoCo2",
		Slots: []Slot{
			{
				Exts: []string{".rom", ".ccc"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  2,
				},
			},
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  3,
				},
			},
			{
				Exts: []string{".cas"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  2,
				},
			},
		},
	},
	"EDSAC": {
		Id:           "EDSAC",
		Name:         "EDSAC",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerCambridge,
		ReleaseDate:  "1949-05-06",
		Folder:       []string{"EDSAC"},
		Rbf:          "_Computer/EDSAC",
		Slots: []Slot{
			{
				Exts: []string{".tap"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"Galaksija": {
		Id:          "Galaksija",
		Name:        "Galaksija",
		Category:    CategoryComputer,
		ReleaseDate: "1983-01-01",
		Folder:      []string{"Galaksija"},
		Rbf:         "_Computer/Galaksija",
		Slots: []Slot{
			{
				Exts: []string{".tap"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Interact": {
		Id:           "Interact",
		Name:         "Interact",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerInteract,
		ReleaseDate:  "1981-01-01",
		Folder:       []string{"Interact"},
		Rbf:          "_Computer/Interact",
		Slots: []Slot{
			{
				Exts: []string{".cin", ".k7"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Jupiter": {
		Id:           "Jupiter",
		Name:         "Jupiter Ace",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerJupiter,
		ReleaseDate:  "1982-09-22",
		Folder:       []string{"Jupiter"},
		Rbf:          "_Computer/Jupiter",
		Slots: []Slot{
			{
				Exts: []string{".ace"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Laser": {
		Id:           "Laser",
		Name:         "Laser 350 500 700",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerVideoTechnology,
		ReleaseDate:  "1984-01-01",
		Alias:        []string{"Laser310"},
		Folder:       []string{"Laser"},
		Rbf:          "_Computer/Laser310",
		Slots: []Slot{
			{
				Exts: []string{".vz"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"Lynx48": {
		Id:           "Lynx48",
		Name:         "Lynx 48 96K",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerCambridge,
		ReleaseDate:  "1983-03-01",
		Folder:       []string{"Lynx48"},
		Rbf:          "_Computer/Lynx48",
		Slots: []Slot{
			{
				Exts: []string{".tap"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"MacPlus": {
		Id:           "MacPlus",
		Name:         "Macintosh Plus",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerApple,
		ReleaseDate:  "1986-01-16",
		Folder:       []string{"MACPLUS"},
		Rbf:          "_Computer/MacPlus",
		Slots: []Slot{
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  2,
				},
			},
			{
				Exts: []string{".img", ".vhd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"MSX": {
		Id:           "MSX",
		Name:         "MSX",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerMicrosoft,
		ReleaseDate:  "1983-06-01",
		Folder:       []string{"MSX"},
		Rbf:          "_Computer/MSX",
		Slots: []Slot{
			{
				Exts: []string{".vhd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"MSX1": {
		Id:           "MSX1",
		Name:         "MSX1",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerMicrosoft,
		ReleaseDate:  "1983-06-01",
		Folder:       []string{"MSX1"},
		Rbf:          "_Computer/MSX1",
		Slots: []Slot{
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".rom"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  2,
				},
			},
		},
	},
	"MultiComp": {
		Id:       "MultiComp",
		Name:     "MultiComp",
		Category: CategoryComputer,
		Folder:   []string{"MultiComp"},
		Rbf:      "_Computer/MultiComp",
		Slots: []Slot{
			{
				Exts: []string{".img"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"Orao": {
		Id:           "Orao",
		Name:         "Orao",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerPEL,
		ReleaseDate:  "1984-01-01",
		Folder:       []string{"ORAO"},
		Rbf:          "_Computer/ORAO",
		Slots: []Slot{
			{
				Exts: []string{".tap"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Oric": {
		Id:           "Oric",
		Name:         "Oric",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerTangerine,
		ReleaseDate:  "1983-01-01",
		Folder:       []string{"Oric"},
		Rbf:          "_Computer/Oric",
		Slots: []Slot{
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"PCXT": {
		Id:           "PCXT",
		Name:         "PC XT",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerIBM,
		ReleaseDate:  "1983-03-08",
		Folder:       []string{"PCXT"},
		Rbf:          "_Computer/PCXT",
		Slots: []Slot{
			{
				Exts: []string{".img", ".ima", ".vfd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".img", ".ima", ".vfd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
			{
				Exts: []string{".vhd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  2,
				},
			},
			{
				Exts: []string{".vhd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  3,
				},
			},
		},
	},
	"PDP1": {
		Id:           "PDP1",
		Name:         "PDP-1",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerDEC,
		ReleaseDate:  "1960-11-01",
		Folder:       []string{"PDP1"},
		Rbf:          "_Computer/PDP1",
		Slots: []Slot{
			{
				Exts: []string{".pdp", ".rim", ".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"PET2001": {
		Id:           "PET2001",
		Name:         "Commodore PET 2001",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerCommodore,
		ReleaseDate:  "1977-10-01",
		Folder:       []string{"PET2001"},
		Rbf:          "_Computer/PET2001",
		Slots: []Slot{
			{
				Exts: []string{".prg", ".tap"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"PMD85": {
		Id:           "PMD85",
		Name:         "PMD 85-2A",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerTesla,
		ReleaseDate:  "1985-01-01",
		Folder:       []string{"PMD85"},
		Rbf:          "_Computer/PMD85",
		Slots: []Slot{
			{
				Exts: []string{".rmm"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"QL": {
		Id:           "QL",
		Name:         "Sinclair QL",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerSinclair,
		ReleaseDate:  "1984-01-12",
		Folder:       []string{"QL"},
		Rbf:          "_Computer/QL",
		Slots: []Slot{
			{
				Exts: []string{".win"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".mdv"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  2,
				},
			},
		},
	},
	"RX78": {
		Id:           "RX78",
		Name:         "RX-78 Gundam",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerBandai,
		ReleaseDate:  "1983-01-01",
		Folder:       []string{"RX78"},
		Rbf:          "_Computer/RX78",
		Slots: []Slot{
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"SAMCoupe": {
		Id:           "SAMCoupe",
		Name:         "SAM Coupe",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerMilesGordon,
		ReleaseDate:  "1989-12-01",
		Folder:       []string{"SAMCOUPE"},
		Rbf:          "_Computer/SAMCoupe",
		Slots: []Slot{
			{
				Exts: []string{".dsk", ".mgt", ".img"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".dsk", ".mgt", ".img"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
		},
	},
	"SordM5": {
		Id:           "SordM5",
		Name:         "M5",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerSord,
		ReleaseDate:  "1982-01-01",
		Alias:        []string{"Sord M5"},
		Folder:       []string{"Sord M5"},
		Rbf:          "_Computer/SordM5",
		Slots: []Slot{
			{
				Exts: []string{".bin", ".rom"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
			{
				Exts: []string{".cas"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  2,
				},
			},
		},
	},
	"Specialist": {
		Id:       "Specialist",
		Name:     "Specialist MX",
		Category: CategoryComputer,
		Alias:    []string{"SPMX"},
		Folder:   []string{"SPMX"},
		Rbf:      "_Computer/Specialist",
		Slots: []Slot{
			{
				Exts: []string{".rks"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
			{
				Exts: []string{".odi"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"SVI328": {
		Id:           "SVI328",
		Name:         "SV-328",
		Folder:       []string{"SVI328"},
		Category:     CategoryComputer,
		Manufacturer: ManufacturerSpectravideo,
		ReleaseDate:  "1983-01-01",
		Rbf:          "_Computer/Svi328",
		Slots: []Slot{
			{
				Exts: []string{".bin", ".rom"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
			{
				Exts: []string{".cas"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  2,
				},
			},
		},
	},
	"TatungEinstein": {
		Id:           "TatungEinstein",
		Name:         "Tatung Einstein",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerTatung,
		ReleaseDate:  "1984-06-01",
		Folder:       []string{"TatungEinstein"},
		Rbf:          "_Computer/TatungEinstein",
		Slots: []Slot{
			{
				Exts: []string{".dsk"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"TI994A": {
		Id:           "TI994A",
		Name:         "TI-99 4A",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerTexasInstruments,
		ReleaseDate:  "1981-06-01",
		Alias:        []string{"TI-99_4A"},
		Folder:       []string{"TI-99_4A"},
		Rbf:          "_Computer/Ti994a",
		Slots: []Slot{
			{
				Exts: []string{".m99", ".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  2,
				},
			},
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  3,
				},
			},
		},
	},
	"TomyTutor": {
		Id:           "TomyTutor",
		Name:         "Tutor",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerTomy,
		ReleaseDate:  "1983-01-01",
		Folder:       []string{"TomyTutor"},
		Rbf:          "_Computer/TomyTutor",
		Slots: []Slot{
			{
				Exts: []string{".bin"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  2,
				},
			},
		},
	},
	"TRS80": {
		Id:           "TRS80",
		Name:         "TRS-80",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerTandy,
		ReleaseDate:  "1977-08-03",
		Folder:       []string{"TRS-80"},
		Rbf:          "_Computer/TRS-80",
		Slots: []Slot{
			{
				Exts: []string{".dsk", ".jv1"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
			{
				Exts: []string{".cmd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  2,
				},
			},
			{
				Exts: []string{".cas"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"TSConf": {
		Id:       "TSConf",
		Name:     "TS-Config",
		Category: CategoryComputer,
		Folder:   []string{"TSConf"},
		Rbf:      "_Computer/TSConf",
		Slots: []Slot{
			{
				Exts: []string{".vhd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"UK101": {
		Id:           "UK101",
		Name:         "UK101",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerCompukit,
		ReleaseDate:  "1979-01-01",
		Folder:       []string{"UK101"},
		Rbf:          "_Computer/UK101",
		Slots: []Slot{
			{
				Exts: []string{".txt", ".bas", ".lod"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Vector06C": {
		Id:          "Vector06C",
		Name:        "Vector-06C",
		Category:    CategoryComputer,
		ReleaseDate: "1986-01-01",
		Alias:       []string{"Vector06"},
		Folder:      []string{"VECTOR06"},
		Rbf:         "_Computer/Vector-06C",
		Slots: []Slot{
			{
				Exts: []string{".rom", ".com", ".c00", ".edd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
			{
				Exts: []string{".fdd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
		},
	},
	"VIC20": {
		Id:           "VIC20",
		Name:         "Commodore VIC-20",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerCommodore,
		ReleaseDate:  "1981-05-01",
		Folder:       []string{"VIC20"},
		Rbf:          "_Computer/VIC20",
		Slots: []Slot{
			{
				Exts: []string{".d64", ".g64"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
			{
				Exts: []string{".prg", ".crt", ".ct?", ".tap"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"X68000": {
		Id:           "X68000",
		Name:         "X68000",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerSharp,
		ReleaseDate:  "1987-03-28",
		Folder:       []string{"X68000", "_Computer/_X68000 Games"},
		Rbf:          "_Computer/X68000",
		Slots: []Slot{
			{
				Exts: []string{".d88"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".d88"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
			{
				Exts: []string{".hdf", ".mgl"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  2,
				},
			},
			{
				Exts: []string{".ram"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  3,
				},
			},
		},
	},
	"ZX81": {
		Id:           "ZX81",
		Name:         "ZX81",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerSinclair,
		ReleaseDate:  "1981-03-05",
		Folder:       []string{"ZX81"},
		Rbf:          "_Computer/ZX81",
		Slots: []Slot{
			{
				Exts: []string{".o", ".p"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"ZXNext": {
		Id:           "ZXNext",
		Name:         "ZX Spectrum Next",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerSpecNext,
		ReleaseDate:  "2020-02-01",
		Folder:       []string{"ZXNext"},
		Rbf:          "_Computer/ZXNext",
		Slots: []Slot{
			{
				Exts: []string{".vhd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".vhd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
			{
				Exts: []string{".tzx", ".csw"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
	"ZXSpectrum": {
		Id:           "ZXSpectrum",
		Name:         "ZX Spectrum",
		Category:     CategoryComputer,
		Manufacturer: ManufacturerSinclair,
		ReleaseDate:  "1982-04-23",
		Alias:        []string{"Spectrum"},
		Folder:       []string{"Spectrum"},
		Rbf:          "_Computer/ZX-Spectrum",
		Slots: []Slot{
			{
				Exts: []string{".trd", ".img", ".dsk", ".mgt"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  0,
				},
			},
			{
				Exts: []string{".tap", ".csw", ".tzx"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  2,
				},
			},
			{
				Exts: []string{".z80", ".sna"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  4,
				},
			},
			{
				Exts: []string{".vhd"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "s",
					Index:  1,
				},
			},
		},
	},
	// Other
	"Arcade": {
		Id:           "Arcade",
		Name:         "Arcade Cores",
		Category:     CategoryArcade,
		Manufacturer: ManufacturerMiSTer,
		Folder:       []string{"_Arcade"},
		Slots: []Slot{
			{
				Exts: []string{".mra"},
				Mgl:  nil,
			},
		},
	},
	"Arduboy": {
		Id:       "Arduboy",
		Name:     "Arduboy",
		Category: CategoryOther,
		Folder:   []string{"Arduboy"},
		Rbf:      "_Other/Arduboy",
		Slots: []Slot{
			{
				Exts: []string{".bin", ".hex"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Chip8": {
		Id:       "Chip8",
		Name:     "CHIP-8",
		Category: CategoryOther,
		Folder:   []string{"Chip8"},
		Rbf:      "_Other/Chip8",
		Slots: []Slot{
			{
				Exts: []string{".ch8"},
				Mgl: &MglParams{
					Delay:  1,
					Method: "f",
					Index:  0,
				},
			},
		},
	},
	"Groovy": {
		Id:          "Groovy",
		Name:        "Groovy",
		Category:    CategoryOther,
		ReleaseDate: "2024-03-02",
		Alias:       []string{"Groovy"},
		Folder:      []string{"Groovy"},
		Rbf:         "_Utility/Groovy",
		Slots: []Slot{
			{
				Exts: []string{".gmc"},
				Mgl: &MglParams{
					Delay:  3,
					Method: "f",
					Index:  1,
				},
			},
		},
	},
}
