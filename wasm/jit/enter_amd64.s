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
TEXT ·enter(SB), NOSPLIT, $0-20
	MOVQ code+0(FP), R11
	MOVQ ctx+8(FP), DI
	CALL R11
	MOVL AX, ret+16(FP)
	RET
