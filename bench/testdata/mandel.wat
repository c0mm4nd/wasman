;; float-heavy kernel: total escape-time iterations over an n x n grid
(module
  (func (export "mandel") (param $n i32) (result i32)
    (local $px i32) (local $py i32) (local $i i32) (local $cnt i32)
    (local $x f64) (local $y f64) (local $cx f64) (local $cy f64) (local $xx f64)
    (block $done
      (loop $rows
        (br_if $done (i32.ge_s (local.get $py) (local.get $n)))
        (local.set $px (i32.const 0))
        (block $rowdone
          (loop $cols
            (br_if $rowdone (i32.ge_s (local.get $px) (local.get $n)))
            (local.set $cx (f64.sub
              (f64.div (f64.mul (f64.convert_i32_s (local.get $px)) (f64.const 3.5))
                       (f64.convert_i32_s (local.get $n)))
              (f64.const 2.5)))
            (local.set $cy (f64.sub
              (f64.div (f64.mul (f64.convert_i32_s (local.get $py)) (f64.const 2.0))
                       (f64.convert_i32_s (local.get $n)))
              (f64.const 1.0)))
            (local.set $x (f64.const 0)) (local.set $y (f64.const 0))
            (local.set $i (i32.const 0))
            (block $iterdone
              (loop $iter
                (br_if $iterdone (i32.ge_s (local.get $i) (i32.const 32)))
                (br_if $iterdone (f64.gt
                  (f64.add (f64.mul (local.get $x) (local.get $x))
                           (f64.mul (local.get $y) (local.get $y)))
                  (f64.const 4)))
                (local.set $xx (f64.add
                  (f64.sub (f64.mul (local.get $x) (local.get $x))
                           (f64.mul (local.get $y) (local.get $y)))
                  (local.get $cx)))
                (local.set $y (f64.add
                  (f64.mul (f64.mul (local.get $x) (local.get $y)) (f64.const 2))
                  (local.get $cy)))
                (local.set $x (local.get $xx))
                (local.set $cnt (i32.add (local.get $cnt) (i32.const 1)))
                (local.set $i (i32.add (local.get $i) (i32.const 1)))
                (br $iter)))
            (local.set $px (i32.add (local.get $px) (i32.const 1)))
            (br $cols)))
        (local.set $py (i32.add (local.get $py) (i32.const 1)))
        (br $rows)))
    (local.get $cnt)))
