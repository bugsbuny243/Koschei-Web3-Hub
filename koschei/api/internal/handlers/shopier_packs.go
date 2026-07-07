package handlers

type shopierPack struct {
	AmountTRY int
}

var shopierPacks = map[string]shopierPack{
	"starter":      {AmountTRY: 0},
	"builder":      {AmountTRY: 0},
	"professional": {AmountTRY: 0},
	"studio":       {AmountTRY: 0},
	"enterprise":   {AmountTRY: 0},
}
