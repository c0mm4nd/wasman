//go:build (darwin || linux) && amd64

#include "textflag.h"

// func enter(code uintptr, ctx *Ctx) uint32
//
// Generated code contract (only caller-saved registers, DF clear):
//   DI = *Ctx on entry; status result in AX on exit
//   SI = operand stack base   R8 = locals base
//   R9 = memory base          R10 = memory length
//   AX, CX, DX, R11 = scratch
// The generated prologue loads SI/R8/R9/R10 from Ctx, the epilogue stores
// the final stack index into Ctx and sets the status in AX, then RET.
// The zeroed AX is the sentinel "software link register": native calls
// pass the return address in AX and save it in the callee frame, so a
// zero tells an epilogue to return through the hardware stack (balancing
// this trampoline's CALL). SI seeds the entry frame's stack base.
TEXT ·enter(SB), NOSPLIT, $0-20
	MOVQ code+0(FP), R11
	MOVQ ctx+8(FP), DI
	XORL AX, AX
	MOVQ 0(DI), SI
	CALL R11
	MOVL AX, ret+16(FP)
	RET
