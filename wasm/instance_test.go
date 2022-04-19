package wasm

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"

	"github.com/c0mm4nd/wasman/utils"

	"github.com/c0mm4nd/wasman/expr"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
)

func TestInstance_executeConstExpression(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, expression := range []*expr.Expression{
			{OpCode: 0xa},
			{OpCode: expr.OpCodeGlobalGet, Data: []byte{0x2}},
		} {
			m := &Module{IndexSpace: new(IndexSpace)}
			ins := &Instance{Module: m}
			_, err := ins.execExpr(expression)
			if err == nil {
				t.Log(err)
				t.Fail()
			}
		}
	})

	t.Run("ok", func(t *testing.T) {
		for _, c := range []struct {
			ins  Instance
			expr *expr.Expression
			val  interface{}
		}{
			{
				expr: &expr.Expression{
					OpCode: expr.OpCodeI64Const,
					Data:   []byte{0x5},
				},
				val: int64(5),
			},
			{
				expr: &expr.Expression{
					OpCode: expr.OpCodeI32Const,
					Data:   []byte{0x5},
				},
				val: int32(5),
			},
			{
				expr: &expr.Expression{
					OpCode: expr.OpCodeF32Const,
					Data:   []byte{0x40, 0xe1, 0x47, 0x40},
				},
				val: float32(3.1231232),
			},
			{
				expr: &expr.Expression{
					OpCode: expr.OpCodeF64Const,
					Data:   []byte{0x5e, 0xc4, 0xd8, 0xf9, 0x27, 0xfc, 0x08, 0x40},
				},
				val: 3.1231231231,
			},
		} {

			actual, err := c.ins.execExpr(c.expr)
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.val, actual) {
				t.Fail()
			}
		}
	})
}

func TestModule_resolveImports(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, c := range []struct {
			module        *Module
			externModules map[string]*Module
		}{
			{
				module: &Module{ImportSection: []*segments.ImportSegment{
					{Module: "a", Name: "b"},
				}},
			},
			{
				module: &Module{ImportSection: []*segments.ImportSegment{
					{Module: "a", Name: "b"},
				}},
				externModules: map[string]*Module{
					"a": {},
				},
			},
			{
				module: &Module{ImportSection: []*segments.ImportSegment{
					{Module: "a", Name: "b", Desc: &segments.ImportDesc{}},
				}},
				externModules: map[string]*Module{
					"a": {ExportSection: map[string]*segments.ExportSegment{
						"b": {
							Name: "a",
							Desc: &segments.ExportDesc{Kind: 1},
						},
					}},
				},
			},
		} {
			ins := &Instance{Module: c.module}
			err := ins.resolveImports(c.externModules)
			if err == nil {
				t.Fail()
			}
			t.Log(err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		m := &Module{
			ImportSection: []*segments.ImportSegment{
				{Module: "a", Name: "b", Desc: &segments.ImportDesc{Kind: 0x03}},
			},
			IndexSpace: new(IndexSpace),
		}
		ems := map[string]*Module{
			"a": {
				ExportSection: map[string]*segments.ExportSegment{
					"b": {
						Name: "a",
						Desc: &segments.ExportDesc{Kind: 0x03},
					},
				},
				IndexSpace: &IndexSpace{
					Globals: []*Global{{
						GlobalType: &types.GlobalType{},
						Val:        1,
					}},
				},
			},
		}

		ins := &Instance{Module: m}
		err := ins.resolveImports(ems)
		if err != nil {
			t.Fail()
		}
		if m.IndexSpace.Globals[0].Val != 1 {
			t.Fail()
		}
	})
}

func TestModule_applyFunctionImport(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		m := &Module{
			TypeSection: []*types.FuncType{{ReturnTypes: []types.ValueType{types.ValueTypeF64}}},
			IndexSpace:  new(IndexSpace),
		}
		is := &segments.ImportSegment{Desc: &segments.ImportDesc{TypeIndexPtr: utils.Uint32Ptr(0)}}
		em := &Module{IndexSpace: &IndexSpace{Functions: []fn{
			&wasmFunc{
				signature: &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeF64}}},
		}}}
		es := &segments.ExportSegment{Desc: &segments.ExportDesc{}}
		ins := &Instance{Module: m}
		err := ins.applyFunctionImport(is, em, es)
		if err != nil {
			t.Fail()
		}
		if em.IndexSpace.Functions[0] != m.IndexSpace.Functions[0] {
			t.Fail()
		}
	})

	t.Run("error", func(t *testing.T) {
		for _, c := range []struct {
			module          Module
			importSegment   *segments.ImportSegment
			exportedModule  *Module
			exportedSegment *segments.ExportSegment
		}{
			{
				module:          Module{IndexSpace: new(IndexSpace)},
				exportedModule:  &Module{IndexSpace: new(IndexSpace)},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{Index: 10}},
			},
			{
				module:          Module{IndexSpace: new(IndexSpace)},
				exportedModule:  &Module{IndexSpace: new(IndexSpace)},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{}},
			},
			{
				module:          Module{TypeSection: []*types.FuncType{{InputTypes: []types.ValueType{types.ValueTypeF64}}}},
				importSegment:   &segments.ImportSegment{Desc: &segments.ImportDesc{TypeIndexPtr: utils.Uint32Ptr(0)}},
				exportedModule:  &Module{IndexSpace: &IndexSpace{Functions: []fn{&wasmFunc{signature: &types.FuncType{}}}}},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{}},
			},
			{
				module:          Module{TypeSection: []*types.FuncType{{ReturnTypes: []types.ValueType{types.ValueTypeF64}}}},
				importSegment:   &segments.ImportSegment{Desc: &segments.ImportDesc{TypeIndexPtr: utils.Uint32Ptr(0)}},
				exportedModule:  &Module{IndexSpace: &IndexSpace{Functions: []fn{&wasmFunc{signature: &types.FuncType{}}}}},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{}},
			},
			{
				module:        Module{TypeSection: []*types.FuncType{{}}},
				importSegment: &segments.ImportSegment{Desc: &segments.ImportDesc{TypeIndexPtr: utils.Uint32Ptr(0)}},
				exportedModule: &Module{IndexSpace: &IndexSpace{Functions: []fn{&wasmFunc{
					signature: &types.FuncType{InputTypes: []types.ValueType{types.ValueTypeF64}}}},
				}},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{}},
			},
			{
				module:        Module{TypeSection: []*types.FuncType{{}}},
				importSegment: &segments.ImportSegment{Desc: &segments.ImportDesc{TypeIndexPtr: utils.Uint32Ptr(0)}},
				exportedModule: &Module{IndexSpace: &IndexSpace{Functions: []fn{&wasmFunc{
					signature: &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeF64}}}},
				}},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{}},
			},
		} {
			err := (&Instance{Module: &c.module}).applyFunctionImport(c.importSegment, c.exportedModule, c.exportedSegment)
			if err == nil {
				t.Fail()
			}
		}
	})
}

func TestModule_applyTableImport(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		es := &segments.ExportSegment{Desc: &segments.ExportDesc{Index: 10}}
		em := &Module{IndexSpace: new(IndexSpace)}
		err := (&Instance{Module: &Module{}}).applyTableImport(em, es)
		if err == nil {
			t.Fail()
		}
	})

	t.Run("ok", func(t *testing.T) {
		es := &segments.ExportSegment{Desc: &segments.ExportDesc{}}

		var exp uint32 = 10
		em := &Module{
			IndexSpace: &IndexSpace{Tables: []*Table{
				{Value: []*uint32{&exp}},
			}},
		}

		m := &Module{IndexSpace: new(IndexSpace)}
		ins := &Instance{Module: m}
		err := ins.applyTableImport(em, es)
		if err != nil {
			t.Fail()
		}
		if *ins.Module.IndexSpace.Tables[0].Value[0] != exp {
			t.Fail()
		}
	})
}

func TestModule_applyMemoryImport(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		es := &segments.ExportSegment{Desc: &segments.ExportDesc{Index: 10}}
		em := &Module{IndexSpace: new(IndexSpace)}
		err := (&Instance{Module: &Module{}}).applyMemoryImport(em, es)
		if err == nil {
			t.Fail()
		}
	})

	t.Run("ok", func(t *testing.T) {
		es := &segments.ExportSegment{Desc: &segments.ExportDesc{}}
		em := &Module{
			IndexSpace: &IndexSpace{Memories: []*Memory{{Value: []byte{0x01}}}},
		}
		m := &Module{IndexSpace: new(IndexSpace)}
		ins := &Instance{Module: m}
		err := ins.applyMemoryImport(em, es)
		if err != nil {
			t.Fail()
		}
		if byte(0x01) != ins.Module.IndexSpace.Memories[0].Value[0] {
			t.Fail()
		}
	})
}

func TestModule_applyGlobalImport(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, c := range []struct {
			exportedModule  *Module
			exportedSegment *segments.ExportSegment
		}{
			{
				exportedModule:  &Module{IndexSpace: new(IndexSpace)},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{Index: 10}},
			},
			{
				exportedModule: &Module{IndexSpace: &IndexSpace{Globals: []*Global{{
					GlobalType: &types.GlobalType{
						Mutable: true,
					},
				}}}},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{}},
			},
		} {
			if (&Instance{Module: &Module{}}).applyGlobalImport(c.exportedModule, c.exportedSegment) == nil {
				t.Fail()
			}
		}
	})

	t.Run("ok", func(t *testing.T) {
		m := &Module{IndexSpace: new(IndexSpace)}
		em := &Module{
			IndexSpace: &IndexSpace{
				Globals: []*Global{{GlobalType: &types.GlobalType{}, Val: 1}},
			},
		}
		es := &segments.ExportSegment{Desc: &segments.ExportDesc{}}

		ins := &Instance{Module: m}
		err := ins.applyGlobalImport(em, es)
		if err != nil {
			t.Fail()
		}
		if ins.IndexSpace.Globals[0].Val != 1 {
			t.Fail()
		}
	})
}

func TestModule_buildGlobalIndexSpace(t *testing.T) {
	m := &Module{
		GlobalSection: []*segments.GlobalSegment{
			{
				Type: nil,
				Init: &expr.Expression{
					OpCode: expr.OpCodeI64Const,
					Data:   []byte{0x01},
				},
			},
		},
		IndexSpace: new(IndexSpace),
	}
	ins := &Instance{Module: m}
	err := ins.buildGlobalIndexSpace()
	if err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(&Global{GlobalType: nil, Val: int64(1)}, m.IndexSpace.Globals[0]) {
		t.Fail()
	}
}

func TestModule_buildFunctionIndexSpace(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, m := range []*Module{
			{
				FunctionSection: []uint32{1000},
				IndexSpace:      new(IndexSpace),
			},
			{
				FunctionSection: []uint32{0},
				TypeSection:     []*types.FuncType{{}},
				IndexSpace:      new(IndexSpace)},
		} {
			if (&Instance{Module: m}).buildFunctionIndexSpace() == nil {
				t.Fail()
			}
		}
	})

	t.Run("ok", func(t *testing.T) {
		m := &Module{
			TypeSection:     []*types.FuncType{{ReturnTypes: []types.ValueType{types.ValueTypeF32}}},
			FunctionSection: []uint32{0},
			CodeSection:     []*segments.CodeSegment{{Body: []byte{0x01}}},
			IndexSpace:      new(IndexSpace),
		}
		ins := &Instance{Module: m}
		if ins.buildFunctionIndexSpace() != nil {
			t.Fail()
		}
		f := m.IndexSpace.Functions[0].(*wasmFunc)
		if f.signature.ReturnTypes[0] != types.ValueTypeF32 {
			t.Fail()
		}
		if f.body[0] != 0x01 {
			t.Fail()
		}
	})
}

func TestModule_buildMemoryIndexSpace(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, m := range []*Module{
			{DataSection: []*segments.DataSegment{{MemoryIndex: 1}}, IndexSpace: new(IndexSpace)},
			{DataSection: []*segments.DataSegment{{MemoryIndex: 0}}, IndexSpace: &IndexSpace{
				Memories: []*Memory{
					{Value: []byte{}},
				},
			}},

			{
				DataSection:   []*segments.DataSegment{{OffsetExpression: &expr.Expression{}}},
				MemorySection: []*types.MemoryType{{}},
				IndexSpace: &IndexSpace{Memories: []*Memory{
					{Value: []byte{}},
				}},
			},
			{
				DataSection: []*segments.DataSegment{
					{
						OffsetExpression: &expr.Expression{
							OpCode: expr.OpCodeI32Const, Data: []byte{0x01},
						},
						Init: []byte{0x01, 0x02},
					},
				},
				MemorySection: []*types.MemoryType{{Max: utils.Uint32Ptr(0)}},
				IndexSpace: &IndexSpace{Memories: []*Memory{
					{Value: []byte{}},
				}},
			},
		} {
			ins := &Instance{Module: m}
			err := ins.buildMemoryIndexSpace()
			if err == nil {
				t.Fail()
			}
			t.Log(err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		for _, c := range []struct {
			m   *Module
			exp []*Memory
		}{
			{
				m: &Module{
					DataSection: []*segments.DataSegment{
						{
							OffsetExpression: &expr.Expression{
								OpCode: expr.OpCodeI32Const,
								Data:   []byte{0x00},
							},
							Init: []byte{0x01, 0x01},
						},
					},
					MemorySection: []*types.MemoryType{{}},
					IndexSpace: &IndexSpace{Memories: []*Memory{
						{Value: []byte{}},
					}},
				},
				exp: []*Memory{{Value: []byte{0x01, 0x01}}},
			},
			{
				m: &Module{
					DataSection: []*segments.DataSegment{
						{
							OffsetExpression: &expr.Expression{
								OpCode: expr.OpCodeI32Const,
								Data:   []byte{0x00},
							},
							Init: []byte{0x01, 0x01},
						},
					},
					MemorySection: []*types.MemoryType{{}},
					IndexSpace: &IndexSpace{Memories: []*Memory{
						{
							Value: []byte{0x00, 0x00, 0x00},
						},
					}},
				},
				exp: []*Memory{{Value: []byte{0x01, 0x01, 0x00}}},
			},
			{
				m: &Module{
					DataSection: []*segments.DataSegment{
						{
							OffsetExpression: &expr.Expression{
								OpCode: expr.OpCodeI32Const,
								Data:   []byte{0x01},
							},
							Init: []byte{0x01, 0x01},
						},
					},
					MemorySection: []*types.MemoryType{{}},
					IndexSpace: &IndexSpace{Memories: []*Memory{
						{Value: []byte{0x00, 0x00, 0x00}},
					}},
				},
				exp: []*Memory{{Value: []byte{0x00, 0x01, 0x01}}},
			},
			{
				m: &Module{
					DataSection: []*segments.DataSegment{
						{
							OffsetExpression: &expr.Expression{
								OpCode: expr.OpCodeI32Const,
								Data:   []byte{0x02},
							},
							Init: []byte{0x01, 0x01},
						},
					},
					MemorySection: []*types.MemoryType{{}},
					IndexSpace: &IndexSpace{Memories: []*Memory{
						{Value: []byte{0x00, 0x00, 0x00}},
					}},
				},
				exp: []*Memory{{Value: []byte{0x00, 0x00, 0x01, 0x01}}},
			},
			{
				m: &Module{
					DataSection: []*segments.DataSegment{
						{
							OffsetExpression: &expr.Expression{
								OpCode: expr.OpCodeI32Const,
								Data:   []byte{0x01},
							},
							Init: []byte{0x01, 0x01},
						},
					},
					MemorySection: []*types.MemoryType{{}},
					IndexSpace: &IndexSpace{Memories: []*Memory{
						{Value: []byte{0x00, 0x00, 0x00, 0x00}},
					}},
				},
				exp: []*Memory{{Value: []byte{0x00, 0x01, 0x01, 0x00}}},
			},
			{
				m: &Module{
					DataSection: []*segments.DataSegment{
						{
							OffsetExpression: &expr.Expression{
								OpCode: expr.OpCodeI32Const,
								Data:   []byte{0x01},
							},
							Init:        []byte{0x01, 0x01},
							MemoryIndex: 1,
						},
					},
					MemorySection: []*types.MemoryType{{}, {}},
					IndexSpace: &IndexSpace{Memories: []*Memory{
						{Value: []byte{}},
						{Value: []byte{0x00, 0x00, 0x00, 0x00}},
					}},
				},
				exp: []*Memory{{Value: []byte{}}, {Value: []byte{0x00, 0x01, 0x01, 0x00}}},
			},
		} {
			ins := &Instance{Module: c.m}
			err := ins.buildMemoryIndexSpace()
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, ins.IndexSpace.Memories) {
				t.Fail()
			}
		}
	})
}

func TestModule_buildTableIndexSpace(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, m := range []*Module{
			{
				ElementsSection: []*segments.ElemSegment{{TableIndex: 10}},
				IndexSpace:      new(IndexSpace),
			},
			{
				ElementsSection: []*segments.ElemSegment{{TableIndex: 0}},
				IndexSpace: &IndexSpace{Tables: []*Table{
					{Value: []*uint32{}},
				}},
			},
			{
				ElementsSection: []*segments.ElemSegment{{TableIndex: 0, OffsetExpr: &expr.Expression{}}},
				TableSection:    []*types.TableType{{}},
				IndexSpace: &IndexSpace{Tables: []*Table{
					{Value: []*uint32{}},
				}},
			},
			{
				ElementsSection: []*segments.ElemSegment{{
					TableIndex: 0,
					OffsetExpr: &expr.Expression{
						OpCode: expr.OpCodeI32Const,
						Data:   []byte{0x0},
					},
					Init: []uint32{0x0, 0x0},
				}},
				TableSection: []*types.TableType{{Limits: &types.Limits{
					Max: utils.Uint32Ptr(1),
				}}},
				IndexSpace: &IndexSpace{Tables: []*Table{
					{Value: []*uint32{}},
				}},
			},
		} {
			err := (&Instance{Module: m}).buildTableIndexSpace()
			if err == nil {
				t.Fail()
			}
			t.Log(err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		for _, c := range []struct {
			m   *Module
			exp []*Table
		}{
			{
				m: &Module{
					ElementsSection: []*segments.ElemSegment{{
						TableIndex: 0,
						OffsetExpr: &expr.Expression{
							OpCode: expr.OpCodeI32Const,
							Data:   []byte{0x0},
						},
						Init: []uint32{0x1, 0x1},
					}},
					TableSection: []*types.TableType{{Limits: &types.Limits{}}},
					IndexSpace: &IndexSpace{Tables: []*Table{
						{Value: []*uint32{}},
					}},
				},
				exp: []*Table{
					{Value: []*uint32{utils.Uint32Ptr(0x01), utils.Uint32Ptr(0x01)}},
				},
			},
			{
				m: &Module{
					ElementsSection: []*segments.ElemSegment{{
						TableIndex: 0,
						OffsetExpr: &expr.Expression{
							OpCode: expr.OpCodeI32Const,
							Data:   []byte{0x0},
						},
						Init: []uint32{0x1, 0x1},
					}},
					TableSection: []*types.TableType{{Limits: &types.Limits{}}},
					IndexSpace: &IndexSpace{
						Tables: []*Table{
							{Value: []*uint32{utils.Uint32Ptr(0x0), utils.Uint32Ptr(0x0)}}},
					},
				},
				exp: []*Table{
					{Value: []*uint32{utils.Uint32Ptr(0x01), utils.Uint32Ptr(0x01)}},
				},
			},
			{
				m: &Module{
					ElementsSection: []*segments.ElemSegment{{
						TableIndex: 0,
						OffsetExpr: &expr.Expression{
							OpCode: expr.OpCodeI32Const,
							Data:   []byte{0x1},
						},
						Init: []uint32{0x1, 0x1},
					}},
					TableSection: []*types.TableType{{Limits: &types.Limits{}}},
					IndexSpace: &IndexSpace{
						Tables: []*Table{
							{Value: []*uint32{nil, utils.Uint32Ptr(0x0), utils.Uint32Ptr(0x0)}},
						},
					},
				},
				exp: []*Table{
					{Value: []*uint32{nil, utils.Uint32Ptr(0x01), utils.Uint32Ptr(0x01)}},
				},
			},
			{
				m: &Module{
					ElementsSection: []*segments.ElemSegment{{
						TableIndex: 0,
						OffsetExpr: &expr.Expression{
							OpCode: expr.OpCodeI32Const,
							Data:   []byte{0x1},
						},
						Init: []uint32{0x1},
					}},
					TableSection: []*types.TableType{{Limits: &types.Limits{}}},
					IndexSpace: &IndexSpace{
						Tables: []*Table{
							{Value: []*uint32{nil, nil, nil}},
						},
					},
				},
				exp: []*Table{
					{Value: []*uint32{nil, utils.Uint32Ptr(0x01), nil}},
				},
			},
		} {
			ins := &Instance{Module: c.m}
			err := ins.buildTableIndexSpace()
			if err != nil {
				t.Fail()
			}
			if len(ins.Module.IndexSpace.Tables) != len(c.exp) {
				t.Fail()
			}
			for i, actualTable := range ins.Module.IndexSpace.Tables {
				expTable := c.exp[i]
				if len(actualTable.Value) != len(expTable.Value) {
					t.Fail()
				}
				for i, exp := range expTable.Value {
					if exp == nil {
						if actualTable.Value[i] != nil {
							t.Fail()
						}
					} else {
						if *actualTable.Value[i] != *exp {
							t.Fail()
						}
					}
				}
			}
		}
	})
}
func TestModule_readBlockType(t *testing.T) {
	for _, c := range []struct {
		bytes []byte
		exp   *types.FuncType
	}{
		{bytes: []byte{0x40}, exp: &types.FuncType{}},
		{bytes: []byte{0x7f}, exp: &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeI32}}},
		{bytes: []byte{0x7e}, exp: &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeI64}}},
		{bytes: []byte{0x7d}, exp: &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeF32}}},
		{bytes: []byte{0x7c}, exp: &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeF64}}},
	} {
		actual, num, err := (&Instance{Module: &Module{}}).readBlockType(bytes.NewBuffer(c.bytes))
		if err != nil {
			t.Fail()
		}
		if num != 1 {
			t.Fail()
		}
		if !reflect.DeepEqual(c.exp, actual) {
			t.Fail()
		}
	}

	m := &Module{TypeSection: []*types.FuncType{{}, {InputTypes: []types.ValueType{types.ValueTypeI32}}}}
	actual, num, err := (&Instance{Module: m}).readBlockType(bytes.NewBuffer([]byte{0x01}))
	if err != nil {
		t.Fail()
	}
	if num != 1 {
		t.Fail()
	}
	if !reflect.DeepEqual(&types.FuncType{InputTypes: []types.ValueType{types.ValueTypeI32}}, actual) {
		t.Fail()
	}
}

func TestModule_parseBlocks(t *testing.T) {
	m := &Module{TypeSection: []*types.FuncType{{}, {}}}
	for i, c := range []struct {
		body []byte
		exp  map[uint64]*funcBlock
	}{
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, 0x0, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          3,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeI32Load), 0x00, 0x0, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          5,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeI64Store32), 0x00, 0x0, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          5,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeMemoryGrow), 0x00, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          4,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeMemorySize), 0x00, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          4,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeI32Const), 0x02, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          4,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeI64Const), 0x02, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          4,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1,
				byte(expr.OpCodeF32Const), 0x02, 0x02, 0x02, 0x02,
				byte(expr.OpCodeEnd),
			},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          7,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1,
				byte(expr.OpCodeF64Const), 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
				byte(expr.OpCodeEnd),
			},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          11,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeLocalGet), 0x02, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          4,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeGlobalSet), 0x03, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          4,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeGlobalSet), 0x03, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          4,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeBr), 0x03, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          4,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeBrIf), 0x03, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          4,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeCall), 0x03, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          4,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeCallIndirect), 0x03, 0x00, byte(expr.OpCodeEnd)},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          5,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeBrTable),
				0x03, 0x01, 0x01, 0x01, 0x01, byte(expr.OpCodeEnd),
			},
			exp: map[uint64]*funcBlock{
				0: {
					StartAt:        0,
					EndAt:          8,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeNop),
				byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeCallIndirect), 0x03, 0x00, byte(expr.OpCodeEnd),
				byte(expr.OpCodeIf), 0x1, byte(expr.OpCodeLocalGet), 0x02,
				byte(expr.OpCodeElse), byte(expr.OpCodeLocalGet), 0x02,
				byte(expr.OpCodeEnd),
			},
			exp: map[uint64]*funcBlock{
				1: {
					StartAt:        1,
					EndAt:          6,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
				7: {
					StartAt:        7,
					ElseAt:         11,
					EndAt:          14,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
		{
			body: []byte{byte(expr.OpCodeNop),
				byte(expr.OpCodeBlock), 0x1, byte(expr.OpCodeCallIndirect), 0x03, 0x00, byte(expr.OpCodeEnd),
				byte(expr.OpCodeIf), 0x1, byte(expr.OpCodeLocalGet), 0x02,
				byte(expr.OpCodeElse), byte(expr.OpCodeLocalGet), 0x02,
				byte(expr.OpCodeIf), 0x01, byte(expr.OpCodeLocalGet), 0x02, byte(expr.OpCodeEnd),
				byte(expr.OpCodeEnd),
			},
			exp: map[uint64]*funcBlock{
				1: {
					StartAt:        1,
					EndAt:          6,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
				7: {
					StartAt:        7,
					ElseAt:         11,
					EndAt:          19,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
				14: {
					StartAt:        14,
					EndAt:          18,
					BlockType:      &types.FuncType{},
					BlockTypeBytes: 1,
				},
			},
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual, err := (&Instance{Module: m}).parseBlocks(c.body)
			if err != nil {
				t.Fail()
			}
			if !reflect.DeepEqual(c.exp, actual) {
				t.Fail()
			}
		})
	}
}
