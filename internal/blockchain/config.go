package blockchain

const (
	// GenesisPreviousHash is the fixed previous-hash value required for the genesis block.
	GenesisPreviousHash = "0000000000000000000000000000000000000000000000000000000000000000"

	// The genesis nonce/hash were precomputed from the canonical block payload at height 0,
	// timestamp 0, no transactions. This keeps genesis deterministic while still satisfying
	// proof-of-work for all supported difficulties up to MaxDifficulty.
	GenesisNonce uint64 = 47296
	GenesisHash         = "00000ced59ade982305577d2c37a075a180e0b1e0c86566febff9bd2f4320a49"

	FaucetAccount = "FAUCET"

	DefaultDifficulty = 3
	DefaultMaxBlockTx = 5
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
	if c.Difficulty < 0 || c.Difficulty > MaxDifficulty {
		return ErrInvalidDifficulty
	}
	if c.MaxBlockTx <= 0 {
		return ErrBlockSizeOutOfRange
	}
	return nil
}
