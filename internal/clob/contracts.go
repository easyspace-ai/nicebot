package clob

type ContractConfig struct {
	Exchange          string
	Collateral        string
	ConditionalTokens string
}

func GetContractConfig(chainID int64, negRisk bool) (ContractConfig, error) {
	if negRisk {
		switch chainID {
		case 137:
			return ContractConfig{
				Exchange:          "0xC5d563A36AE78145C45a50134d48A1215220f80a",
				Collateral:        "0x2791bca1f2de4661ed88a30c99a7a9449aa84174",
				ConditionalTokens: "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045",
			}, nil
		case 80002:
			return ContractConfig{
				Exchange:          "0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296",
				Collateral:        "0x9c4e1703476e875070ee25b56a58b008cfb8fa78",
				ConditionalTokens: "0x69308FB512518e39F9b16112fA8d994F4e2Bf8bB",
			}, nil
		}
	} else {
		switch chainID {
		case 137:
			return ContractConfig{
				Exchange:          "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E",
				Collateral:        "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174",
				ConditionalTokens: "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045",
			}, nil
		case 80002:
			return ContractConfig{
				Exchange:          "0xdFE02Eb6733538f8Ea35D585af8DE5958AD99E40",
				Collateral:        "0x9c4e1703476e875070ee25b56a58b008cfb8fa78",
				ConditionalTokens: "0x69308FB512518e39F9b16112fA8d994F4e2Bf8bB",
			}, nil
		}
	}
	return ContractConfig{}, ErrInvalidChainID
}
