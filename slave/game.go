package slave

type Game struct {
	BoardID string
	Result  GameResult
}

type Board struct {
	BoardID string
}

type GameResult struct {
	MasterScore int
	SlaveScore  int
	GameHistory []GameHistoryEntry
}

type PostResult struct {
	MasterScore  int
	SlaveScore   int
	MasterSymbol string
	SlaveSymbol  string
}

type GameHistoryEntry struct {
	MasterSymbol string
	SlaveSymbol  string
}

type GameSymbol struct {
	Symbol string
}
