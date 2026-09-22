package config

// Clone returns a deep copy so the server can mutate config under a lock
// (copy-on-write) without touching the currently-served snapshot.
func (c *Config) Clone() *Config {
	cp := *c
	// Settings is a value copy, but its slice field would still share a backing
	// array with the live config — and updateConfig mutates the clone before
	// swapping it in, so readers would see half-applied edits.
	cp.Settings.Check.RetryDelays = append([]int(nil), c.Settings.Check.RetryDelays...)
	cp.Services = make([]Service, len(c.Services))
	copy(cp.Services, c.Services)
	for i := range c.Services {
		if es := c.Services[i].Check.ExpectedStatus; es != nil {
			cp.Services[i].Check.ExpectedStatus = append([]int(nil), es...)
		}
		if rd := c.Services[i].Check.RetryDelays; rd != nil {
			cp.Services[i].Check.RetryDelays = append([]int(nil), rd...)
		}
	}
	return &cp
}
