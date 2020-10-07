package wasman

import (
	"bytes"
	"github.com/c0mm4nd/wasman/instr"
	"github.com/c0mm4nd/wasman/segments"
	"github.com/c0mm4nd/wasman/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestInstance_executeConstExpression(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, expr := range []*instr.Expr{
			{OpCode: 0xa},
			{OpCode: instr.OpCodeGlobalGet, Data: []byte{0x2}},
		} {
			m := &Module{indexSpace: new(indexSpace)}
			ins := &Instance{Module: m}
			_, err := ins.execExpr(expr)
			assert.Error(t, err)
			t.Log(err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		for _, c := range []struct {
			ins  Instance
			expr *instr.Expr
			val  interface{}
		}{
			{
				expr: &instr.Expr{
					OpCode: instr.OpCodeI64Const,
					Data:   []byte{0x5},
				},
				val: int64(5),
			},
			{
				expr: &instr.Expr{
					OpCode: instr.OpCodeI32Const,
					Data:   []byte{0x5},
				},
				val: int32(5),
			},
			{
				expr: &instr.Expr{
					OpCode: instr.OpCodeF32Const,
					Data:   []byte{0x40, 0xe1, 0x47, 0x40},
				},
				val: float32(3.1231232),
			},
			{
				expr: &instr.Expr{
					OpCode: instr.OpCodeF64Const,
					Data:   []byte{0x5e, 0xc4, 0xd8, 0xf9, 0x27, 0xfc, 0x08, 0x40},
				},
				val: 3.1231231231,
			},
		} {

			actual, err := c.ins.execExpr(c.expr)
			require.NoError(t, err)
			assert.Equal(t, c.val, actual)
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
				module: &Module{ImportsSection: []*segments.ImportSegment{
					{Module: "a", Name: "b"},
				}},
			},
			{
				module: &Module{ImportsSection: []*segments.ImportSegment{
					{Module: "a", Name: "b"},
				}},
				externModules: map[string]*Module{
					"a": {},
				},
			},
			{
				module: &Module{ImportsSection: []*segments.ImportSegment{
					{Module: "a", Name: "b", Desc: &segments.ImportDesc{}},
				}},
				externModules: map[string]*Module{
					"a": {ExportsSection: map[string]*segments.ExportSegment{
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
			assert.Error(t, err)
			t.Log(err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		m := &Module{
			ImportsSection: []*segments.ImportSegment{
				{Module: "a", Name: "b", Desc: &segments.ImportDesc{Kind: 0x03}},
			},
			indexSpace: new(indexSpace),
		}
		ems := map[string]*Module{
			"a": {
				ExportsSection: map[string]*segments.ExportSegment{
					"b": {
						Name: "a",
						Desc: &segments.ExportDesc{Kind: 0x03},
					},
				},
				indexSpace: &indexSpace{
					Globals: []*global{{
						Type: &types.GlobalType{},
						Val:  1,
					}},
				},
			},
		}

		ins := &Instance{Module: m}
		err := ins.resolveImports(ems)
		require.NoError(t, err)
		assert.Equal(t, 1, m.indexSpace.Globals[0].Val)
	})
}

func uint32Ptr(u uint32) *uint32 {
	return &u
}

func TestModule_applyFunctionImport(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		m := &Module{
			TypesSection: []*types.FuncType{{ReturnTypes: []types.ValueType{types.ValueTypeF64}}},
			indexSpace:   new(indexSpace),
		}
		is := &segments.ImportSegment{Desc: &segments.ImportDesc{TypeIndexPtr: uint32Ptr(0)}}
		em := &Module{indexSpace: &indexSpace{Functions: []fn{
			&wasmFunc{
				signature: &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeF64}}},
		}}}
		es := &segments.ExportSegment{Desc: &segments.ExportDesc{}}
		ins := &Instance{Module: m}
		err := ins.applyFunctionImport(is, em, es)
		require.NoError(t, err)
		assert.Equal(t, em.indexSpace.Functions[0], m.indexSpace.Functions[0])
	})

	t.Run("error", func(t *testing.T) {
		for _, c := range []struct {
			module          Module
			importSegment   *segments.ImportSegment
			exportedModule  *Module
			exportedSegment *segments.ExportSegment
		}{
			{
				module:          Module{indexSpace: new(indexSpace)},
				exportedModule:  &Module{indexSpace: new(indexSpace)},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{Index: 10}},
			},
			{
				module:          Module{indexSpace: new(indexSpace)},
				exportedModule:  &Module{indexSpace: new(indexSpace)},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{}},
			},
			{
				module:          Module{TypesSection: []*types.FuncType{{InputTypes: []types.ValueType{types.ValueTypeF64}}}},
				importSegment:   &segments.ImportSegment{Desc: &segments.ImportDesc{TypeIndexPtr: uint32Ptr(0)}},
				exportedModule:  &Module{indexSpace: &indexSpace{Functions: []fn{&wasmFunc{signature: &types.FuncType{}}}}},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{}},
			},
			{
				module:          Module{TypesSection: []*types.FuncType{{ReturnTypes: []types.ValueType{types.ValueTypeF64}}}},
				importSegment:   &segments.ImportSegment{Desc: &segments.ImportDesc{TypeIndexPtr: uint32Ptr(0)}},
				exportedModule:  &Module{indexSpace: &indexSpace{Functions: []fn{&wasmFunc{signature: &types.FuncType{}}}}},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{}},
			},
			{
				module:        Module{TypesSection: []*types.FuncType{{}}},
				importSegment: &segments.ImportSegment{Desc: &segments.ImportDesc{TypeIndexPtr: uint32Ptr(0)}},
				exportedModule: &Module{indexSpace: &indexSpace{Functions: []fn{&wasmFunc{
					signature: &types.FuncType{InputTypes: []types.ValueType{types.ValueTypeF64}}}},
				}},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{}},
			},
			{
				module:        Module{TypesSection: []*types.FuncType{{}}},
				importSegment: &segments.ImportSegment{Desc: &segments.ImportDesc{TypeIndexPtr: uint32Ptr(0)}},
				exportedModule: &Module{indexSpace: &indexSpace{Functions: []fn{&wasmFunc{
					signature: &types.FuncType{ReturnTypes: []types.ValueType{types.ValueTypeF64}}}},
				}},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{}},
			},
		} {
			assert.Error(t, (&Instance{Module: &c.module}).applyFunctionImport(c.importSegment, c.exportedModule, c.exportedSegment))
		}
	})
}

func TestModule_applyTableImport(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		es := &segments.ExportSegment{Desc: &segments.ExportDesc{Index: 10}}
		em := &Module{indexSpace: new(indexSpace)}
		err := (&Instance{Module: &Module{}}).applyTableImport(em, es)
		assert.Error(t, err)
	})

	t.Run("ok", func(t *testing.T) {
		es := &segments.ExportSegment{Desc: &segments.ExportDesc{}}

		var exp uint32 = 10
		em := &Module{
			indexSpace: &indexSpace{Tables: [][]*uint32{{&exp}}},
		}

		m := &Module{indexSpace: new(indexSpace)}
		ins := &Instance{Module: m}
		err := ins.applyTableImport(em, es)
		require.NoError(t, err)
		assert.Equal(t, exp, *ins.Module.indexSpace.Tables[0][0])
	})
}

func TestModule_applyMemoryImport(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		es := &segments.ExportSegment{Desc: &segments.ExportDesc{Index: 10}}
		em := &Module{indexSpace: new(indexSpace)}
		err := (&Instance{Module: &Module{}}).applyMemoryImport(em, es)
		assert.Error(t, err)
	})

	t.Run("ok", func(t *testing.T) {
		es := &segments.ExportSegment{Desc: &segments.ExportDesc{}}
		em := &Module{
			indexSpace: &indexSpace{Memories: [][]byte{{0x01}}},
		}
		m := &Module{indexSpace: new(indexSpace)}
		ins := &Instance{Module: m}
		err := ins.applyMemoryImport(em, es)
		require.NoError(t, err)
		assert.Equal(t, byte(0x01), ins.Module.indexSpace.Memories[0][0])
	})
}

func TestModule_applyGlobalImport(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, c := range []struct {
			exportedModule  *Module
			exportedSegment *segments.ExportSegment
		}{
			{
				exportedModule:  &Module{indexSpace: new(indexSpace)},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{Index: 10}},
			},
			{
				exportedModule: &Module{indexSpace: &indexSpace{Globals: []*global{{Type: &types.GlobalType{
					Mutable: true,
				}}}}},
				exportedSegment: &segments.ExportSegment{Desc: &segments.ExportDesc{}},
			},
		} {
			assert.Error(t, (&Instance{Module: &Module{}}).applyGlobalImport(c.exportedModule, c.exportedSegment))
		}
	})

	t.Run("ok", func(t *testing.T) {
		m := &Module{indexSpace: new(indexSpace)}
		em := &Module{
			indexSpace: &indexSpace{
				Globals: []*global{{Type: &types.GlobalType{}, Val: 1}},
			},
		}
		es := &segments.ExportSegment{Desc: &segments.ExportDesc{}}

		ins := &Instance{Module: m}
		err := ins.applyGlobalImport(em, es)
		require.NoError(t, err)
		assert.Equal(t, 1, ins.indexSpace.Globals[0].Val)
	})
}

func TestModule_buildGlobalIndexSpace(t *testing.T) {
	m := &Module{
		GlobalsSection: []*segments.GlobalSegment{
			{
				Type: nil,
				Init: &instr.Expr{
					OpCode: instr.OpCodeI64Const,
					Data:   []byte{0x01},
				},
			},
		},
		indexSpace: new(indexSpace),
	}
	ins := &Instance{Module: m}
	require.NoError(t, ins.buildGlobalIndexSpace())
	assert.Equal(t, &global{Type: nil, Val: int64(1)}, m.indexSpace.Globals[0])
}

func TestModule_buildFunctionIndexSpace(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, m := range []*Module{
			{
				FunctionsSection: []uint32{1000},
				indexSpace:       new(indexSpace),
			},
			{
				FunctionsSection: []uint32{0},
				TypesSection:     []*types.FuncType{{}},
				indexSpace:       new(indexSpace)},
		} {
			assert.Error(t, (&Instance{Module: m}).buildFunctionIndexSpace())
		}
	})

	t.Run("ok", func(t *testing.T) {
		m := &Module{
			TypesSection:     []*types.FuncType{{ReturnTypes: []types.ValueType{types.ValueTypeF32}}},
			FunctionsSection: []uint32{0},
			CodesSection:     []*segments.CodeSegment{{Body: []byte{0x01}}},
			indexSpace:       new(indexSpace),
		}
		ins := &Instance{Module: m}
		assert.NoError(t, ins.buildFunctionIndexSpace())
		f := m.indexSpace.Functions[0].(*wasmFunc)
		assert.Equal(t, types.ValueTypeF32, f.signature.ReturnTypes[0])
		assert.Equal(t, byte(0x01), f.body[0])
	})
}

func TestModule_buildMemoryIndexSpace(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, m := range []*Module{
			{DataSection: []*segments.DataSegment{{MemoryIndex: 1}}, indexSpace: new(indexSpace)},
			{DataSection: []*segments.DataSegment{{MemoryIndex: 0}}, indexSpace: &indexSpace{
				Memories: [][]byte{{}},
			}},

			{
				DataSection:   []*segments.DataSegment{{OffsetExpression: &instr.Expr{}}},
				MemorySection: []*types.MemoryType{{}},
				indexSpace:    &indexSpace{Memories: [][]byte{{}}},
			},
			{
				DataSection: []*segments.DataSegment{
					{
						OffsetExpression: &instr.Expr{
							OpCode: instr.OpCodeI32Const, Data: []byte{0x01},
						},
						Init: []byte{0x01, 0x02},
					},
				},
				MemorySection: []*types.MemoryType{{Max: uint32Ptr(0)}},
				indexSpace:    &indexSpace{Memories: [][]byte{{}}},
			},
		} {
			ins := &Instance{Module: m}
			err := ins.buildMemoryIndexSpace()
			assert.Error(t, err)
			t.Log(err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		for _, c := range []struct {
			m   *Module
			exp [][]byte
		}{
			{
				m: &Module{
					DataSection: []*segments.DataSegment{
						{
							OffsetExpression: &instr.Expr{
								OpCode: instr.OpCodeI32Const,
								Data:   []byte{0x00},
							},
							Init: []byte{0x01, 0x01},
						},
					},
					MemorySection: []*types.MemoryType{{}},
					indexSpace:    &indexSpace{Memories: [][]byte{{}}},
				},
				exp: [][]byte{{0x01, 0x01}},
			},
			{
				m: &Module{
					DataSection: []*segments.DataSegment{
						{
							OffsetExpression: &instr.Expr{
								OpCode: instr.OpCodeI32Const,
								Data:   []byte{0x00},
							},
							Init: []byte{0x01, 0x01},
						},
					},
					MemorySection: []*types.MemoryType{{}},
					indexSpace:    &indexSpace{Memories: [][]byte{{0x00, 0x00, 0x00}}},
				},
				exp: [][]byte{{0x01, 0x01, 0x00}},
			},
			{
				m: &Module{
					DataSection: []*segments.DataSegment{
						{
							OffsetExpression: &instr.Expr{
								OpCode: instr.OpCodeI32Const,
								Data:   []byte{0x01},
							},
							Init: []byte{0x01, 0x01},
						},
					},
					MemorySection: []*types.MemoryType{{}},
					indexSpace:    &indexSpace{Memories: [][]byte{{0x00, 0x00, 0x00}}},
				},
				exp: [][]byte{{0x00, 0x01, 0x01}},
			},
			{
				m: &Module{
					DataSection: []*segments.DataSegment{
						{
							OffsetExpression: &instr.Expr{
								OpCode: instr.OpCodeI32Const,
								Data:   []byte{0x02},
							},
							Init: []byte{0x01, 0x01},
						},
					},
					MemorySection: []*types.MemoryType{{}},
					indexSpace:    &indexSpace{Memories: [][]byte{{0x00, 0x00, 0x00}}},
				},
				exp: [][]byte{{0x00, 0x00, 0x01, 0x01}},
			},
			{
				m: &Module{
					DataSection: []*segments.DataSegment{
						{
							OffsetExpression: &instr.Expr{
								OpCode: instr.OpCodeI32Const,
								Data:   []byte{0x01},
							},
							Init: []byte{0x01, 0x01},
						},
					},
					MemorySection: []*types.MemoryType{{}},
					indexSpace:    &indexSpace{Memories: [][]byte{{0x00, 0x00, 0x00, 0x00}}},
				},
				exp: [][]byte{{0x00, 0x01, 0x01, 0x00}},
			},
			{
				m: &Module{
					DataSection: []*segments.DataSegment{
						{
							OffsetExpression: &instr.Expr{
								OpCode: instr.OpCodeI32Const,
								Data:   []byte{0x01},
							},
							Init:        []byte{0x01, 0x01},
							MemoryIndex: 1,
						},
					},
					MemorySection: []*types.MemoryType{{}, {}},
					indexSpace:    &indexSpace{Memories: [][]byte{{}, {0x00, 0x00, 0x00, 0x00}}},
				},
				exp: [][]byte{{}, {0x00, 0x01, 0x01, 0x00}},
			},
		} {
			ins := &Instance{Module: c.m}
			require.NoError(t, ins.buildMemoryIndexSpace())
			assert.Equal(t, c.exp, ins.indexSpace.Memories)
		}
	})
}

func TestModule_buildTableIndexSpace(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		for _, m := range []*Module{
			{
				ElementsSection: []*segments.ElemSegment{{TableIndex: 10}},
				indexSpace:      new(indexSpace),
			},
			{
				ElementsSection: []*segments.ElemSegment{{TableIndex: 0}},
				indexSpace:      &indexSpace{Tables: [][]*uint32{{}}},
			},
			{
				ElementsSection: []*segments.ElemSegment{{TableIndex: 0, OffsetExpr: &instr.Expr{}}},
				TablesSection:   []*types.TableType{{}},
				indexSpace:      &indexSpace{Tables: [][]*uint32{{}}},
			},
			{
				ElementsSection: []*segments.ElemSegment{{
					TableIndex: 0,
					OffsetExpr: &instr.Expr{
						OpCode: instr.OpCodeI32Const,
						Data:   []byte{0x0},
					},
					Init: []uint32{0x0, 0x0},
				}},
				TablesSection: []*types.TableType{{Limits: &types.Limits{
					Max: uint32Ptr(1),
				}}},
				indexSpace: &indexSpace{Tables: [][]*uint32{{}}},
			},
		} {
			err := (&Instance{Module: m}).buildTableIndexSpace()
			assert.Error(t, err)
			t.Log(err)
		}
	})

	t.Run("ok", func(t *testing.T) {
		for _, c := range []struct {
			m   *Module
			exp [][]*uint32
		}{
			{
				m: &Module{
					ElementsSection: []*segments.ElemSegment{{
						TableIndex: 0,
						OffsetExpr: &instr.Expr{
							OpCode: instr.OpCodeI32Const,
							Data:   []byte{0x0},
						},
						Init: []uint32{0x1, 0x1},
					}},
					TablesSection: []*types.TableType{{Limits: &types.Limits{}}},
					indexSpace:    &indexSpace{Tables: [][]*uint32{{}}},
				},
				exp: [][]*uint32{{uint32Ptr(0x01), uint32Ptr(0x01)}},
			},
			{
				m: &Module{
					ElementsSection: []*segments.ElemSegment{{
						TableIndex: 0,
						OffsetExpr: &instr.Expr{
							OpCode: instr.OpCodeI32Const,
							Data:   []byte{0x0},
						},
						Init: []uint32{0x1, 0x1},
					}},
					TablesSection: []*types.TableType{{Limits: &types.Limits{}}},
					indexSpace: &indexSpace{
						Tables: [][]*uint32{{uint32Ptr(0x0), uint32Ptr(0x0)}},
					},
				},
				exp: [][]*uint32{{uint32Ptr(0x01), uint32Ptr(0x01)}},
			},
			{
				m: &Module{
					ElementsSection: []*segments.ElemSegment{{
						TableIndex: 0,
						OffsetExpr: &instr.Expr{
							OpCode: instr.OpCodeI32Const,
							Data:   []byte{0x1},
						},
						Init: []uint32{0x1, 0x1},
					}},
					TablesSection: []*types.TableType{{Limits: &types.Limits{}}},
					indexSpace: &indexSpace{
						Tables: [][]*uint32{{nil, uint32Ptr(0x0), uint32Ptr(0x0)}},
					},
				},
				exp: [][]*uint32{{nil, uint32Ptr(0x01), uint32Ptr(0x01)}},
			},
			{
				m: &Module{
					ElementsSection: []*segments.ElemSegment{{
						TableIndex: 0,
						OffsetExpr: &instr.Expr{
							OpCode: instr.OpCodeI32Const,
							Data:   []byte{0x1},
						},
						Init: []uint32{0x1},
					}},
					TablesSection: []*types.TableType{{Limits: &types.Limits{}}},
					indexSpace: &indexSpace{
						Tables: [][]*uint32{{nil, nil, nil}},
					},
				},
				exp: [][]*uint32{{nil, uint32Ptr(0x01), nil}},
			},
		} {
			ins := &Instance{Module: c.m}
			require.NoError(t, ins.buildTableIndexSpace())
			require.Len(t, ins.Module.indexSpace.Tables, len(c.exp))
			for i, actualTable := range ins.Module.indexSpace.Tables {
				expTable := c.exp[i]
				require.Len(t, actualTable, len(expTable))
				for i, exp := range expTable {
					if exp == nil {
						assert.Nil(t, actualTable[i])
					} else {
						assert.Equal(t, *exp, *actualTable[i])
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
		require.NoError(t, err)
		assert.Equal(t, uint64(1), num)
		assert.Equal(t, c.exp, actual)
	}

	m := &Module{TypesSection: []*types.FuncType{{}, {InputTypes: []types.ValueType{types.ValueTypeI32}}}}
	actual, num, err := (&Instance{Module: m}).readBlockType(bytes.NewBuffer([]byte{0x01}))
	require.NoError(t, err)
	assert.Equal(t, uint64(1), num)
	assert.Equal(t, &types.FuncType{InputTypes: []types.ValueType{types.ValueTypeI32}}, actual)
}

func TestModule_parseBlocks(t *testing.T) {
	m := &Module{TypesSection: []*types.FuncType{{}, {}}}
	for i, c := range []struct {
		body []byte
		exp  map[uint64]*funcBlock
	}{
		{
			body: []byte{byte(instr.OpCodeBlock), 0x1, 0x0, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeI32Load), 0x00, 0x0, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeI64Store32), 0x00, 0x0, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeMemoryGrow), 0x00, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeMemorySize), 0x00, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeI32Const), 0x02, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeI64Const), 0x02, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1,
				byte(instr.OpCodeF32Const), 0x02, 0x02, 0x02, 0x02,
				byte(instr.OpCodeEnd),
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
			body: []byte{byte(instr.OpCodeBlock), 0x1,
				byte(instr.OpCodeF64Const), 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
				byte(instr.OpCodeEnd),
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeLocalGet), 0x02, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeGlobalSet), 0x03, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeGlobalSet), 0x03, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeBr), 0x03, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeBrIf), 0x03, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeCall), 0x03, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeCallIndirect), 0x03, 0x00, byte(instr.OpCodeEnd)},
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
			body: []byte{byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeBrTable),
				0x03, 0x01, 0x01, 0x01, 0x01, byte(instr.OpCodeEnd),
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
			body: []byte{byte(instr.OpCodeNop),
				byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeCallIndirect), 0x03, 0x00, byte(instr.OpCodeEnd),
				byte(instr.OpCodeIf), 0x1, byte(instr.OpCodeLocalGet), 0x02,
				byte(instr.OpCodeElse), byte(instr.OpCodeLocalGet), 0x02,
				byte(instr.OpCodeEnd),
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
			body: []byte{byte(instr.OpCodeNop),
				byte(instr.OpCodeBlock), 0x1, byte(instr.OpCodeCallIndirect), 0x03, 0x00, byte(instr.OpCodeEnd),
				byte(instr.OpCodeIf), 0x1, byte(instr.OpCodeLocalGet), 0x02,
				byte(instr.OpCodeElse), byte(instr.OpCodeLocalGet), 0x02,
				byte(instr.OpCodeIf), 0x01, byte(instr.OpCodeLocalGet), 0x02, byte(instr.OpCodeEnd),
				byte(instr.OpCodeEnd),
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
			require.NoError(t, err)
			assert.Equal(t, c.exp, actual)
		})
	}
}
