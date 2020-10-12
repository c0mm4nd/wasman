package segments

type Kind = byte

// available export kinds
const (
	KindFunction Kind = 0x00
	KindTable    Kind = 0x01
	KindMem      Kind = 0x02
	KindGlobal   Kind = 0x03
)
