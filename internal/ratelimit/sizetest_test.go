package ratelimit

import (
	"testing"
	"unsafe"
)

func TestEntrySizeV2LoadTest(t *testing.T) {
	size := unsafe.Sizeof(entry{})
	t.Logf("unsafe.Sizeof(entry{}) = %d bytes", size)
	if size >= 128 {
		t.Errorf("entry struct size %d >= 128B target (PERF-03)", size)
	}
}
