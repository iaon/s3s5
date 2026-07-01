package tunnel

import "sync/atomic"

type Stats struct {
	chunksSent     atomic.Uint64
	chunksReceived atomic.Uint64
	bytesSent      atomic.Uint64
	bytesReceived  atomic.Uint64
	putObjects     atomic.Uint64
	getObjects     atomic.Uint64
	headObjects    atomic.Uint64
	listObjects    atomic.Uint64
	deleteObjects  atomic.Uint64
	activeSessions atomic.Int64
}

type StatsSnapshot struct {
	ChunksSent     uint64
	ChunksReceived uint64
	BytesSent      uint64
	BytesReceived  uint64
	PutObjects     uint64
	GetObjects     uint64
	HeadObjects    uint64
	ListObjects    uint64
	DeleteObjects  uint64
	ActiveSessions int64
}

func (s *Stats) Snapshot() StatsSnapshot {
	if s == nil {
		return StatsSnapshot{}
	}
	return StatsSnapshot{
		ChunksSent:     s.chunksSent.Load(),
		ChunksReceived: s.chunksReceived.Load(),
		BytesSent:      s.bytesSent.Load(),
		BytesReceived:  s.bytesReceived.Load(),
		PutObjects:     s.putObjects.Load(),
		GetObjects:     s.getObjects.Load(),
		HeadObjects:    s.headObjects.Load(),
		ListObjects:    s.listObjects.Load(),
		DeleteObjects:  s.deleteObjects.Load(),
		ActiveSessions: s.activeSessions.Load(),
	}
}

func (s *Stats) incChunksSent(n int) {
	if s != nil {
		s.chunksSent.Add(1)
		s.bytesSent.Add(uint64(n))
	}
}

func (s *Stats) incChunksReceived(n int) {
	if s != nil {
		s.chunksReceived.Add(1)
		s.bytesReceived.Add(uint64(n))
	}
}

func (s *Stats) incPut() {
	if s != nil {
		s.putObjects.Add(1)
	}
}

func (s *Stats) incGet() {
	if s != nil {
		s.getObjects.Add(1)
	}
}

func (s *Stats) incHead() {
	if s != nil {
		s.headObjects.Add(1)
	}
}

func (s *Stats) incList() {
	if s != nil {
		s.listObjects.Add(1)
	}
}

func (s *Stats) incDelete() {
	if s != nil {
		s.deleteObjects.Add(1)
	}
}

func (s *Stats) incActive() {
	if s != nil {
		s.activeSessions.Add(1)
	}
}

func (s *Stats) decActive() {
	if s != nil {
		s.activeSessions.Add(-1)
	}
}
