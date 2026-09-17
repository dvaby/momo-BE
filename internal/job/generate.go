package job

import (
	"fmt"
	"sync/atomic"
	"time"
)

var counter uint64

// GenerateID membuat job_id unik dengan prefix, timestamp nano, dan counter atomic.
// Contoh output: "soal_1726544806000000000_42"
func GenerateID(prefix string) string {
	n := atomic.AddUint64(&counter, 1)
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), n)
}
