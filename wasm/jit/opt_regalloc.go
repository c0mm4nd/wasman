//go:build (darwin || linux) && (arm64 || amd64)

package jit

import "sort"

// Register allocation for the optimizing tier.
//
// Every vreg has a "home" memory location that needs no bookkeeping at run
// time: locals live in the frame's locals array, stack-boundary vregs in
// their operand-stack slot, temporaries in spill slots placed above the
// wasm stack heights. Host exits (calls, memory.grow) return to Go and
// clobber every machine register, so any interval crossing an exit — and
// anything an exit reads or writes — is simply assigned to its home; with
// that rule a continuation only needs the pinned base registers reloaded,
// never allocatable state. Everything else gets a machine register by
// linear scan.

// assignment for one vreg.
type vloc struct {
	reg   int16 // >= 0: machine register index (into the arch's pool)
	spill int32 // when reg < 0: home slot (see homeOf)
}

type interval struct {
	v          int
	start, end int
	mem        bool // forced to its memory home
}

type allocation struct {
	loc        []vloc // indexed by compact vreg id
	compact    map[int]int
	spillSlots int // temp spill slots used above maxH
}

// homeKind describes where a vreg's home lives.
const (
	homeLocal = iota // locals array [j]
	homeStack        // operand stack slot [i]
	homeSpill        // operand stack slot [maxH + k]
)

func (fn *irFunc) homeOf(v int) (kind, idx int) {
	if v < fn.nlocals {
		return homeLocal, v
	}
	if v < fn.nlocals+maxOptHeight {
		return homeStack, v - fn.nlocals
	}
	return homeSpill, 0 // spill index assigned during allocation
}

// use/def enumeration per instruction.
func (i *irInstr) uses(f func(v int)) {
	if i.a >= 0 {
		f(i.a)
	}
	if i.b >= 0 {
		f(i.b)
	}
	if i.c >= 0 {
		f(i.c)
	}
}

// allocate computes intervals and runs linear scan over nregs registers.
func (fn *irFunc) allocate(nregs int) *allocation {
	type rng struct{ start, end int }
	ranges := map[int]*rng{}
	forced := map[int]bool{}

	touch := func(v, at int) {
		r, ok := ranges[v]
		if !ok {
			ranges[v] = &rng{start: at, end: at}
			return
		}
		if at < r.start {
			r.start = at
		}
		if at > r.end {
			r.end = at
		}
	}

	var exits []int
	for idx := range fn.code {
		ins := &fn.code[idx]
		ins.uses(func(v int) { touch(v, idx) })
		if ins.dst >= 0 {
			touch(ins.dst, idx)
		}
		if ins.op == irCallExit || ins.op == irCallNative {
			exits = append(exits, idx)
			// the call communicates through frame memory: everything at or
			// below the site's stack heights stays home-allocated
			max := 0
			if ins.op == irCallExit {
				site := fn.sites[ins.imm]
				max = site.SpBefore
				if site.SpAfter > max {
					max = site.SpAfter
				}
			} else {
				sp := int(uint32(ins.imm))
				sig := fn.callSig(int(ins.imm >> 32))
				max = sp
				if after := sp - sig.In + sig.Out; after > max {
					max = after
				}
			}
			for i := 0; i < max; i++ {
				forced[fn.stackRegID(i)] = true
				touch(fn.stackRegID(i), idx)
			}
		}
		if ins.op == irRet {
			// returned results are read from stack memory by the host
			for i := 0; i < fn.nrets; i++ {
				forced[fn.stackRegID(i)] = true
				touch(fn.stackRegID(i), idx)
			}
		}
	}

	// locals are defined at entry: the prologue materializes them before
	// instruction 0, so their intervals start at -1. That keeps two locals
	// from sharing a register (the later prologue load would clobber the
	// earlier one) and makes an interval reaching past a host exit at
	// index 0 count as crossing it.
	for v, r := range ranges {
		if v < fn.nlocals {
			r.start = -1
		}
	}

	// loop extension. Two rules keep back edges sound: any value defined
	// before a loop and touched inside stays live through it; and mutable
	// slots — locals and stack-boundary vregs — touched anywhere inside a
	// loop are live around the whole loop (iteration N+1 re-reads what
	// iteration N left, so their tight ranges would lie).
	for changed := true; changed; {
		changed = false
		for _, lp := range fn.loops {
			s, e := lp[0], lp[1]
			for v, r := range ranges {
				overlaps := r.start < e && r.end >= s
				if !overlaps {
					continue
				}
				if v < fn.nlocals+maxOptHeight { // local or boundary vreg
					if r.start > s {
						r.start = s
						changed = true
					}
					if r.end < e {
						r.end = e
						changed = true
					}
				} else if r.start < s && r.end < e { // temp crossing entry
					r.end = e
					changed = true
				}
			}
		}
	}

	// exits clobber all registers: crossing intervals go to their homes
	for v, r := range ranges {
		for _, x := range exits {
			if r.start < x && r.end > x {
				forced[v] = true
				break
			}
		}
	}

	ivs := make([]interval, 0, len(ranges))
	for v, r := range ranges {
		ivs = append(ivs, interval{v: v, start: r.start, end: r.end, mem: forced[v]})
	}
	sort.Slice(ivs, func(i, j int) bool { return ivs[i].start < ivs[j].start })

	al := &allocation{compact: make(map[int]int, len(ivs))}
	for _, iv := range ivs {
		al.compact[iv.v] = len(al.loc)
		al.loc = append(al.loc, vloc{reg: -1, spill: -1})
	}

	// linear scan
	type act struct {
		end int
		reg int16
		ci  int
	}
	var active []act
	free := make([]int16, 0, nregs)
	for r := nregs - 1; r >= 0; r-- {
		free = append(free, int16(r))
	}
	spillCount := 0
	home := func(ci int, v int) {
		kind, _ := fn.homeOf(v)
		if kind == homeSpill {
			al.loc[ci].spill = int32(fn.maxH + spillCount)
			spillCount++
		}
	}
	for _, iv := range ivs {
		ci := al.compact[iv.v]
		// expire finished intervals
		na := active[:0]
		for _, a := range active {
			if a.end < iv.start {
				free = append(free, a.reg)
			} else {
				na = append(na, a)
			}
		}
		active = na
		if iv.mem {
			home(ci, iv.v)
			continue
		}
		if len(free) == 0 {
			// spill the active interval that ends last (or this one)
			far, fi := iv.end, -1
			for i, a := range active {
				if a.end > far {
					far, fi = a.end, i
				}
			}
			if fi < 0 {
				home(ci, iv.v)
				continue
			}
			victim := active[fi]
			al.loc[victim.ci] = vloc{reg: -1, spill: -1}
			home(victim.ci, ivByCompact(ivs, al, victim.ci))
			active[fi] = act{end: iv.end, reg: victim.reg, ci: ci}
			al.loc[ci].reg = victim.reg
			continue
		}
		r := free[len(free)-1]
		free = free[:len(free)-1]
		al.loc[ci].reg = r
		active = append(active, act{end: iv.end, reg: r, ci: ci})
	}
	al.spillSlots = spillCount
	return al
}

func ivByCompact(ivs []interval, al *allocation, ci int) int {
	for _, iv := range ivs {
		if al.compact[iv.v] == ci {
			return iv.v
		}
	}
	return -1
}

func (fn *irFunc) stackRegID(i int) int { return fn.nlocals + i }
