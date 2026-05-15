package dispatcher

type Dispatcher struct{}

func New() *Dispatcher {
	return &Dispatcher{}
}

func (d *Dispatcher) QueueCommand(agentID string, payload []byte) error {
	return nil
}
