//go:build (darwin || linux) && arm64

#include "textflag.h"

// func enter(code uintptr, ctx *Ctx) uint32
//
// Generated code contract (all caller-saved registers, g in R28 untouched):
//   R0  = *Ctx on entry (preserved across native calls)
//   R1  = operand stack base   R2 = stack index (slots)
//   R3  = locals base          R4 = memory base   R5 = memory length
//   R27 = status result on exit
//
// The shim records its link register — the trampoline's continuation —
// into Ctx.TrampRet before jumping into generated code, so a host exit
// from any native call depth can return straight to the trampoline.
TEXT ·enter(SB), NOSPLIT, $0-20
	MOVD code+0(FP), R16
	MOVD ctx+8(FP), R0
	CALL jitshim<>(SB)
	MOVW R27, ret+16(FP)
	RET

TEXT jitshim<>(SB), NOSPLIT, $0-0
	MOVD R30, 64(R0)
	B (R16)
