package provider

type MapRegistry map[string]Adapter

func (r MapRegistry) Get(providerKey string) (Adapter, bool) {
	adapter, ok := r[providerKey]
	return adapter, ok
}
