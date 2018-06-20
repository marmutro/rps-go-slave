package slave

type Game struct {
	BoardID string
	Result  GameResult
}

type GameResult struct {
	MasterScore int
	SlaveScore  int
	GameHistory []GameHistoryEntry
}

type GameHistoryEntry struct {
	MasterSymbol string
	SlaveSymbol  string
}

type GameSymbol struct {
	Symbol string
}
