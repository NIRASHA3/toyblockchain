package blockchain

import (
	"fmt"
	"time"
)

// NextDifficulty returns the difficulty that should be used for the next mined block.
//
// Retargeting is intentionally conservative for this toy chain: difficulty changes
// by at most one level after each completed retarget interval. When retargeting is
// disabled with RetargetInterval == 0, the next block keeps the previous non-genesis
// difficulty, and block 1 uses cfg.Difficulty.
func (c Config) NextDifficulty(chain []Block) (int, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	if len(chain) == 0 {
		return 0, fmt.Errorf("%w: empty chain", ErrInvalidChain)
	}

	base := c.Difficulty
	latest := chain[len(chain)-1]
	if latest.Height > 0 {
		base = latest.Difficulty
	}

	if c.RetargetInterval == 0 || latest.Height == 0 {
		return base, nil
	}

	// Retarget only after a full interval has been mined. For example, with an
	// interval of 5, blocks 1..5 keep the starting difficulty, and block 6 uses
	// the adjustment calculated from blocks 1..5.
	if latest.Height%c.RetargetInterval != 0 || len(chain) <= c.RetargetInterval {
		return base, nil
	}

	startIndex := len(chain) - c.RetargetInterval
	if startIndex < 1 {
		return base, nil
	}

	first := chain[startIndex]
	actual := time.Duration(latest.Timestamp-first.Timestamp) * time.Second
	expected := c.TargetBlockTime * time.Duration(c.RetargetInterval-1)
	return adjustDifficulty(base, actual, expected), nil
}

func adjustDifficulty(current int, actual time.Duration, expected time.Duration) int {
	if expected <= 0 {
		return current
	}
	if actual < expected/2 && current < MaxDifficulty {
		return current + 1
	}
	if actual > expected*2 && current > MinDifficulty {
		return current - 1
	}
	return current
}
