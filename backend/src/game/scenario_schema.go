package game
import (
	"encoding/json"
)
// DATA STRUCTURES AND LOADING
type ObjectivesFile struct {
	Objectives []Objective `json:"objectives"`
	Format     int         `json:"format"`
}
// Scenarios have manifest.json, objectives.json, setup.json, symbols.json, memmap.json
type Objective struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Brief       string     `json:"brief"`
	Requires    []string   `json:"requires,omitempty"` //objectives that must be completed before this one is unlocked
	Points      int        `json:"points,omitempty"`
	Success     Predicate  `json:"success"`
	Fail        *Predicate `json:"fail,omitempty"` //optional if the objective has a failure condition
	Partial     []Partial  `json:"partial,omitempty"`
	Retractable bool       `json:"retractable,omitempty"` //defaults to false, if true, completion can be undone if success becomes false
	Hints       []Hint     `json:"hints,omitempty"`
}
type Predicate struct {
	Op     string          `json:"op"`           //What question to ask
	Of     json.RawMessage `json:"of,omitempty"` //possibility of and, or, and not in the conditions
	Path   string          `json:"path,omitempty"`
	Cmp    string          `json:"cmp,omitempty"` //comparison operator
	Value  any             `json:"value,omitempty"`
	Mask   int             `json:"mask,omitempty"`   //Which bits to compare
	Frames int             `json:"frames,omitempty"` //predicates that are true for a number of frames
	ID     string          `json:"id,omitempty"`
	Match  string          `json:"match,omitempty"`
	Regex  bool            `json:"regex,omitempty"`
	// Next three will connect with introspection channel
	At    *AddressRef `json:"at,omitempty"`
	Len   int         `json:"len,omitempty"`   //length in bytes of data to read
	Width int         `json:"width,omitempty"` //how many bytes make up the integer we are examining
	//Next three have scenario ask question about what player has done
	Verb     string `json:"verb,omitempty"`
	Result   string `json:"result,omitempty"`
	Resource string `json:"resource,omitempty"` // which budget we're checking ("writes", "reads")
	Lang     string `json:"lang,omitempty"`
	Entry    string `json:"entry,omitempty"`
}
type AddressRef struct {
	Sym    string `json:"sym,omitempty"`
	Field  string `json:"field,omitempty"`  // address of sym + offset of field
	Offset int    `json:"offset,omitempty"` // address of sym + offset
	Addr   string `json:"addr,omitempty"`   // direct address
}
type Partial struct {
	When Predicate `json:"when"`
	Text string    `json:"text"`
}
type CommandLogEntry struct {
	Verb   string
	Result string
	Addr   uint32
}
type Hint struct {
	AfterFrames int    `json:"after_frames"`
	Text        string `json:"text"`
}
type Manifest struct {
	Format     int            `json:"format"`
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	Revision   int            `json:"revision"`
	Author     string         `json:"author,omitempty"`
	Summary    string         `json:"summary"`
	Difficulty string         `json:"difficulty,omitempty"`
	Impure     bool           `json:"impure,omitempty"`
	Firmware   FirmwareConfig `json:"firmware"` //which firmware files belong to this scenario
	Link       LinkConfig     `json:"link"`
	Console    ConsoleConfig  `json:"console,omitempty"`
	Briefing   string         `json:"briefing"`        // briefing.md
	Setup      string         `json:"setup,omitempty"` // setup.json
	Objectives string         `json:"objectives"`      // objectives.json
	Docs       []string       `json:"docs,omitempty"`
}
type FirmwareConfig struct {
	ROM      string `json:"rom"`
	Symbols  string `json:"symbols"`
	MemMap   string `json:"memmap"`    //valid memory regions
	AppCRC32 string `json:"app_crc32"` //is the right firmware being used
}
type BudgetConfig struct {
	Writes *int `json:"writes,omitempty"`
	Reads  *int `json:"reads,omitempty"`
}
type LinkConfig struct {
	UplinkDelayS    int          `json:"uplink_delay_s"`
	DownlinkDelayS  int          `json:"downlink_delay_s"`
	RequireChecksum *bool        `json:"require_checksum,omitempty"` //whether player commands require checksum, defaults to true
	Budget          BudgetConfig `json:"budget,omitempty"`
}
type ConsoleConfig struct {
	Decode     []string `json:"decode,omitempty"`      //which telemetry channels the console should decode for the player
	DSNComplex string   `json:"dsn_complex,omitempty"` //which DSN complex has the link
}
type Setup struct {
	Format       int          `json:"format"`                  //Scenario format version
	Writes       []SetupWrite `json:"writes"`                  //things to change before playing the scenario
	SettleFrames int          `json:"settle_frames,omitempty"` //how many frames to wait after writes before starting the scenario
}
type SetupWrite struct {
	At   AddressRef `json:"at"`
	U8   *uint8     `json:"u8,omitempty"`
	U16  *uint16    `json:"u16,omitempty"`
	U32  *uint32    `json:"u32,omitempty"`
	Hex  string     `json:"hex,omitempty"`
	Note string     `json:"note,omitempty"`
}
// bridge between human readable scenario objectives and locations in firmware's memory
type SymbolsFile struct {
	Format  int                       `json:"format"`
	Symbols map[string]string         `json:"symbols"` // symbol name to memory address
	Fields  map[string]map[string]int `json:"fields"`  //symbol name to fields to offset
}
// vlaid memory regions for the firmware
type MemMapFile struct {
	Format  int            `json:"format"`
	Regions []MemoryRegion `json:"regions"`
}
type MemoryRegion struct {
	Name string `json:"name"`
	Lo   string `json:"lo"`             // start address
	Hi   string `json:"hi"`             // end address
	Poke string `json:"poke,omitempty"` //write behavior
}
const SupportedScenarioFormat = 1 //the only scenario format we support right now
