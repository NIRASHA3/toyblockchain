package blockchain

const (
	// GenesisPreviousHash is the fixed previous-hash value required for the genesis block.
	GenesisPreviousHash = "0000000000000000000000000000000000000000000000000000000000000000"

	// The genesis nonce/hash are precomputed from the canonical block payload at
	// height 0, timestamp 0, difficulty MaxDifficulty, no transactions.
	// Keeping these constants fixed makes the genesis block tamper-evident.
	GenesisNonce uint64 = 2795095
	GenesisHash         = "0000066df5eeb807e089b751c013567c6909e3e1450129c395c7b024607f6ce0"

	FaucetAccount = "FAUCET"

	DefaultDifficulty = 3
	DefaultMaxBlockTx = 5
	MinDifficulty     = 1
	MaxDifficulty     = 5
)

// Config controls validation, mining, and block assembly.
type Config struct {
	Difficulty int
	MaxBlockTx int
	Workers    int
}

// DefaultConfig returns safe defaults that mine quickly on a laptop.
func DefaultConfig() Config {
	return Config{Difficulty: DefaultDifficulty, MaxBlockTx: DefaultMaxBlockTx, Workers: 0}
}

// Validate checks configuration bounds.
func (c Config) Validate() error {
	if c.Difficulty < MinDifficulty || c.Difficulty > MaxDifficulty {
		return ErrInvalidDifficulty
	}
	if c.MaxBlockTx <= 0 {
		return ErrBlockSizeOutOfRange
	}
	return nil
}
