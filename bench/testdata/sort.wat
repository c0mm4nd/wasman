;; quicksort over linear memory: LCG fill, recursive sort, weighted checksum
(module
  (memory (export "memory") 1)
  (func $part (param $lo i32) (param $hi i32) (result i32)
    (local $p i32) (local $i i32) (local $j i32) (local $t i32)
    (local.set $p (i32.load (i32.shl (local.get $hi) (i32.const 2))))
    (local.set $i (i32.sub (local.get $lo) (i32.const 1)))
    (local.set $j (local.get $lo))
    (block $done
      (loop $scan
        (br_if $done (i32.ge_s (local.get $j) (local.get $hi)))
        (if (i32.le_s (i32.load (i32.shl (local.get $j) (i32.const 2))) (local.get $p))
          (then
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (local.set $t (i32.load (i32.shl (local.get $i) (i32.const 2))))
            (i32.store (i32.shl (local.get $i) (i32.const 2))
                       (i32.load (i32.shl (local.get $j) (i32.const 2))))
            (i32.store (i32.shl (local.get $j) (i32.const 2)) (local.get $t))))
        (local.set $j (i32.add (local.get $j) (i32.const 1)))
        (br $scan)))
    (local.set $i (i32.add (local.get $i) (i32.const 1)))
    (local.set $t (i32.load (i32.shl (local.get $i) (i32.const 2))))
    (i32.store (i32.shl (local.get $i) (i32.const 2))
               (i32.load (i32.shl (local.get $hi) (i32.const 2))))
    (i32.store (i32.shl (local.get $hi) (i32.const 2)) (local.get $t))
    (local.get $i))
  (func $qsort (param $lo i32) (param $hi i32)
    (local $p i32)
    (if (i32.ge_s (local.get $lo) (local.get $hi)) (then (return)))
    (local.set $p (call $part (local.get $lo) (local.get $hi)))
    (call $qsort (local.get $lo) (i32.sub (local.get $p) (i32.const 1)))
    (call $qsort (i32.add (local.get $p) (i32.const 1)) (local.get $hi)))
  (func (export "sort") (param $n i32) (result i64)
    (local $i i32) (local $s i64)
    (block $done
      (loop $fill
        (br_if $done (i32.ge_s (local.get $i) (local.get $n)))
        (i32.store (i32.shl (local.get $i) (i32.const 2))
          (i32.and
            (i32.add (i32.mul (local.get $i) (i32.const 1103515245)) (i32.const 12345))
            (i32.const 0x7fffffff)))
        (local.set $i (i32.add (local.get $i) (i32.const 1)))
        (br $fill)))
    (call $qsort (i32.const 0) (i32.sub (local.get $n) (i32.const 1)))
    (local.set $i (i32.const 0))
    (block $done2
      (loop $sum
        (br_if $done2 (i32.ge_s (local.get $i) (local.get $n)))
        (local.set $s (i64.add (local.get $s)
          (i64.mul (i64.extend_i32_u (i32.load (i32.shl (local.get $i) (i32.const 2))))
                   (i64.extend_i32_s (i32.add (local.get $i) (i32.const 1))))))
        (local.set $i (i32.add (local.get $i) (i32.const 1)))
        (br $sum)))
    (local.get $s)))
