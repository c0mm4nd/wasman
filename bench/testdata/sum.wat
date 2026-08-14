(module
  (func (export "sum") (param i32) (result i64)
    (local $acc i64)
    (block (loop
      (br_if 1 (i32.eqz (local.get 0)))
      (local.set $acc (i64.add (local.get $acc) (i64.extend_i32_u (local.get 0))))
      (local.set 0 (i32.sub (local.get 0) (i32.const 1)))
      (br 0)))
    (local.get $acc)))
