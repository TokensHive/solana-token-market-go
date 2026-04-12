package mathx

func Max[T ~int | ~int64 | ~uint64 | ~float64](a, b T) T {
if a > b {
return a
}
return b
}
