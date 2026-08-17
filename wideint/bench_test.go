package wideint

import "testing"

var sinkU256 U256

func BenchmarkU256Add(b *testing.B) {
	x := U256{0x1111111111111111, 0x2222222222222222, 0x3333333333333333, 0x4444444444444444}
	y := U256{0x5, 0x6, 0x7, 0x8}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkU256 = x.Add(y)
	}
}
func BenchmarkU256Mul(b *testing.B) {
	x := U256{0x1111111111111111, 0x2222222222222222, 0x3333333333333333, 0x4444444444444444}
	y := U256{0x5, 0x6, 0x7, 0x8}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkU256 = x.Mul(y)
	}
}
func BenchmarkU256DivU(b *testing.B) {
	x := U256{0x1111111111111111, 0x2222222222222222, 0x3333333333333333, 0x4444444444444444}
	y := U256{0x1234567, 0x89, 0, 0}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkU256 = x.DivU(y)
	}
}
func BenchmarkU256MulDiv(b *testing.B) {
	x := U256{0x1111111111111111, 0x2222222222222222, 0x3333333333333333, 0x4444444444444444}
	y := U256{0x5555555555555555, 0x6666666666666666, 0x7777777777777777, 0x8888888888888888}
	c := U256{0x1234567890abcdef, 0x42, 0, 0}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkU256, _ = x.MulDiv(y, c)
	}
}
func BenchmarkU256Sqrt(b *testing.B) {
	x := U256{0x1111111111111111, 0x2222222222222222, 0x3333333333333333, 0x4444444444444444}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkU256 = x.Sqrt()
	}
}
