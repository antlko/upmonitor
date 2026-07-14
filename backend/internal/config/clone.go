package config

// Clone returns a deep copy so the server can mutate config under a lock
// (copy-on-write) without touching the currently-served snapshot.
func (c *Config) Clone() *Config {
	cp := *c
	cp.Services = make([]Service, len(c.Services))
	copy(cp.Services, c.Services)
	for i := range c.Services {
		if es := c.Services[i].Check.ExpectedStatus; es != nil {
			cp.Services[i].Check.ExpectedStatus = append([]int(nil), es...)
		}
	}
	return &cp
}
