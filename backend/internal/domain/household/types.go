package household

type ID string
type ResourceCode string

const (
	ResourceProvisions ResourceCode = "provisions"
	ResourceWood       ResourceCode = "wood"
	ResourceTradeGoods ResourceCode = "trade_goods"
	ResourceSilver     ResourceCode = "silver"
)
