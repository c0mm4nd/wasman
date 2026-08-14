(module
  (memory (export "mem") 1)
  (func (export "fillsum") (param i32) (result i32)
    (local $i i32) (local $acc i32)
    (block (loop
      (br_if 1 (i32.ge_u (local.get $i) (local.get 0)))
      (i32.store8 (local.get $i) (local.get $i))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br 0)))
    (local.set $i (i32.const 0))
    (block (loop
      (br_if 1 (i32.ge_u (local.get $i) (local.get 0)))
      (local.set $acc (i32.add (local.get $acc) (i32.load8_u (local.get $i))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br 0)))
    (local.get $acc)))
