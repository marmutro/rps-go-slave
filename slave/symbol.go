package slave

// Symbol enum
type Symbol int

const (
	Rock     Symbol = iota
	Paper           = iota
	Scissors        = iota
)

func (sym Symbol) String() string {
	names := [...]string{
		"Rock",
		"Paper",
		"Scissors"}

	if sym < Rock || sym > Scissors {
		return "Out-of-range"
	}
	return names[sym]
}

func FromString(sym string) Symbol {
	names := [...]string{
		"Rock",
		"Paper",
		"Scissors"}

	i := 0
	for _, name := range names {
		if name == sym {
			return Symbol(i)
		}
		i++
	}
	panic(sym + " not found")
}

func Up(sym Symbol) Symbol {
	if sym < Scissors {
		return sym + 1
	}
	return Rock
}

func Down(sym Symbol) Symbol {
	if sym > Rock {
		return sym - 1
	}
	return Scissors
}
