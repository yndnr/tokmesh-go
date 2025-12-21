// Package benchmark provides performance benchmarks for TokMesh.
//
// Run benchmarks with:
//
//	go test -bench=. -benchmem ./internal/tests/benchmark/...
//
// Run with specific session counts:
//
//	go test -bench=BenchmarkSession -benchmem -benchtime=10s ./internal/tests/benchmark/...
//
// Generate performance report:
//
//	go test -bench=. -benchmem -count=5 ./internal/tests/benchmark/... | tee benchmark.txt
//
// Compare results:
//
//	benchstat old.txt new.txt
package benchmark
