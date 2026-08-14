;; dispatch-dense loop: br_table over four accumulator operations
(module
  (func (export "run") (param $n i32) (result i64)
    (local $i i32) (local $acc i64)
    (local.set $acc (i64.const 7))
    (block $done
      (loop $step
        (br_if $done (i32.ge_u (local.get $i) (local.get $n)))
        (block $next
          (block $c3 (block $c2 (block $c1 (block $c0
            (br_table $c0 $c1 $c2 $c3 (i32.and (local.get $i) (i32.const 3))))
            (local.set $acc (i64.add (local.get $acc) (i64.extend_i32_u (local.get $i))))
            (br $next))
            (local.set $acc (i64.xor (local.get $acc) (i64.const 0x5bf03635)))
            (br $next))
            (local.set $acc (i64.mul (local.get $acc) (i64.const 3)))
            (br $next))
          (local.set $acc (i64.rotl (local.get $acc) (i64.const 9))))
        (local.set $i (i32.add (local.get $i) (i32.const 1)))
        (br $step)))
    (local.get $acc)))
