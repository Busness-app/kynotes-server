package health

type Checker struct{ Ready bool }

func (c *Checker) IsReady() bool { return c != nil && c.Ready }
