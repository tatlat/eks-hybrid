package configenricher

import "context"

type ConfigEnricher interface {
	Enrich(ctx context.Context) error
}
