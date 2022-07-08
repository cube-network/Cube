package baseapp

type ChainI interface {
	LastBlockHeight() int64
}