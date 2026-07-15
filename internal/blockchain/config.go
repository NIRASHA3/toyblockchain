package blockchain

import "time"

const (
	// GenesisPreviousHash is the fixed previous-hash value required for the genesis block.
	GenesisPreviousHash = "0000000000000000000000000000000000000000000000000000000000000000"

	// The genesis nonce/hash are precomputed from the canonical block payload at
	// height 0, timestamp 0, difficulty MaxDifficulty, no transactions, and the empty Merkle root.
	// Keeping these constants fixed makes the genesis block tamper-evident.
	GenesisNonce uint64 = 2417102
	GenesisHash         = "0000050c5cad3e6cb229bb04eacc3c580834a93285f16f6dece119029021fcfd"

	FaucetAccount = "FAUCET"

	DefaultDifficulty       = 3
	DefaultMaxBlockTx       = 5
	DefaultRetargetInterval = 5
	MinDifficulty           = 1
	MaxDifficulty           = 5
)

const DefaultTargetBlockTime = 10 * time.Second

// Config controls validation, mining, difficulty retargeting, and block assembly.
type Config struct {
	Difficulty       int
	MaxBlockTx       int
	Workers          int
	RetargetInterval int
	TargetBlockTime  time.Duration
}

// DefaultConfig returns safe defaults that mine quickly on a laptop.
func DefaultConfig() Config {
	return Config{
		Difficulty:       DefaultDifficulty,
		MaxBlockTx:       DefaultMaxBlockTx,
		Workers:          0,
		RetargetInterval: DefaultRetargetInterval,
		TargetBlockTime:  DefaultTargetBlockTime,
	}
}

// Validate checks configuration bounds.
func (c Config) Validate() error {
	if c.Difficulty < MinDifficulty || c.Difficulty > MaxDifficulty {
		return ErrInvalidDifficulty
	}
	if c.MaxBlockTx <= 0 {
		return ErrBlockSizeOutOfRange
	}
	if c.RetargetInterval < 0 || c.RetargetInterval == 1 {
		return ErrInvalidRetargetConfig
	}
	if c.RetargetInterval > 0 && c.TargetBlockTime <= 0 {
		return ErrInvalidRetargetConfig
	}
	return nil
}
