;; call_indirect-dense loop: a four-entry table applied to an accumulator
(module
  (type $op (func (param i64) (result i64)))
  (table 4 funcref)
  (elem (i32.const 0) $inc $tri $mix $rot)
  (func $inc (type $op) (i64.add (local.get 0) (i64.const 1)))
  (func $tri (type $op) (i64.mul (local.get 0) (i64.const 3)))
  (func $mix (type $op) (i64.xor (local.get 0) (i64.const 0x9e3779b9)))
  (func $rot (type $op) (i64.rotl (local.get 0) (i64.const 7)))
  (func (export "run") (param $n i32) (result i64)
    (local $i i32) (local $acc i64)
    (local.set $acc (i64.const 1))
    (block $done
      (loop $step
        (br_if $done (i32.ge_u (local.get $i) (local.get $n)))
        (local.set $acc (call_indirect (type $op)
          (local.get $acc)
          (i32.and (local.get $i) (i32.const 3))))
        (local.set $i (i32.add (local.get $i) (i32.const 1)))
        (br $step)))
    (local.get $acc)))
