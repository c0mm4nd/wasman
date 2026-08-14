//go:build (darwin || linux) && arm64

#include "textflag.h"

// func enter(code uintptr, ctx *Ctx) uint32
//
// Generated code contract (all caller-saved registers, g in R28 untouched):
//   R0  = *Ctx on entry; status result on exit
//   R1  = operand stack base   R2 = stack index (slots)
//   R3  = locals base          R4 = memory base   R5 = memory length
// The generated prologue loads R1-R5 from Ctx, the epilogue stores the
// stack index back and sets the status in R0, then RET (to LR).
TEXT ·enter(SB), NOSPLIT, $0-20
	MOVD code+0(FP), R16
	MOVD ctx+8(FP), R0
	CALL (R16)
	MOVW R0, ret+16(FP)
	RET
