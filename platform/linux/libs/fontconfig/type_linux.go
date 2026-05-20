package fontconfig

// Config is an opaque handle to a Fontconfig configuration
type Config uintptr

// Pattern is an opaque handle to a Fontconfig pattern
type Pattern uintptr

// Bool is the Fontconfig boolean type
type Bool int32

const (
	False Bool = 0
	True  Bool = 1
)

// Result is the return type for pattern queries
type Result int32

const (
	ResultMatch        Result = 0
	ResultNoMatch      Result = 1
	ResultTypeMismatch Result = 2
	ResultNoId         Result = 3
	ResultOutOfMemory  Result = 4
)

// Object names for FcPatternGet/Add
const (
	Family   = "family"
	Style    = "style"
	Slant    = "slant"
	Weight   = "weight"
	Size     = "size"
	File     = "file"
	Index    = "index"
	Lang     = "lang"
	Spacing  = "spacing"
	Outline  = "outline"
	Scalable = "scalable"
)
