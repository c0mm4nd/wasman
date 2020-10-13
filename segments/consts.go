package segments

// Kind means the types of ​the extern values
// https://www.w3.org/TR/wasm-core-1/#external-values%E2%91%A0
// https://www.w3.org/TR/wasm-core-1/#external-typing%E2%91%A0
type Kind = byte

// available export kinds
const (
	KindFunction Kind = 0x00
	KindTable    Kind = 0x01
	KindMem      Kind = 0x02
	KindGlobal   Kind = 0x03
)
