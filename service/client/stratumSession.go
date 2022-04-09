package client

// IsRunning 检查会话是否在运行（线程安全）
func (ss *StratumSession) IsRunning() bool {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	return ss.isRunning
}

func (ss *StratumSession) Start() (err error) {

	if ss.IsRunning() {
		return
	}

	ss.lock.Lock()
	ss.isRunning = true
	ss.lock.Unlock()

	go ss.runListenMiner()
	return
}
