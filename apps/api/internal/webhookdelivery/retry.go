package webhookdelivery

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"
)

func retryDelay(deliveryID string, sequence int) time.Duration {
	if sequence < 1 {
		sequence = 1
	}
	base := 30 * time.Second
	for i := 1; i < sequence; i++ {
		if base >= 6*time.Hour {
			base = 6 * time.Hour
			break
		}
		base *= 2
	}
	if base > 6*time.Hour {
		base = 6 * time.Hour
	}

	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", deliveryID, sequence)))
	bucket := binary.BigEndian.Uint16(hash[:2])
	// Stable jitter in [90%, 110%], so retries do not synchronize after an outage.
	permille := 900 + int(bucket%201)
	return time.Duration(int64(base) * int64(permille) / 1000)
}
