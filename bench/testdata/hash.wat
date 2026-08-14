;; 64-bit mixing loop: multiply, xor, rotate
(module
  (func (export "hash") (param $n i64) (result i64)
    (local $h i64) (local $i i64)
    (local.set $h (i64.const 0x9E3779B97F4A7C15))
    (block $done
      (loop $mix
        (br_if $done (i64.ge_u (local.get $i) (local.get $n)))
        (local.set $h (i64.xor (local.get $h) (local.get $i)))
        (local.set $h (i64.mul (local.get $h) (i64.const 0xff51afd7ed558ccd)))
        (local.set $h (i64.rotl (local.get $h) (i64.const 31)))
        (local.set $i (i64.add (local.get $i) (i64.const 1)))
        (br $mix)))
    (local.get $h)))
