package configenricher

type ConfigEnricher interface {
	Enrich() error
}
