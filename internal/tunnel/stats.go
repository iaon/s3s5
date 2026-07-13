package tunnel

import (
	"sync"
	"sync/atomic"
	"time"
)

type Stats struct {
	chunksSent     atomic.Uint64
	chunksReceived atomic.Uint64
	bytesSent      atomic.Uint64
	bytesReceived  atomic.Uint64
	sealedSent     atomic.Uint64
	sealedReceived atomic.Uint64
	putObjects     atomic.Uint64
	getObjects     atomic.Uint64
	headObjects    atomic.Uint64
	listObjects    atomic.Uint64
	deleteObjects  atomic.Uint64
	activeSessions atomic.Int64
	started        atomic.Uint64
	opened         atomic.Uint64
	rejected       atomic.Uint64
	completed      atomic.Uint64
	failed         atomic.Uint64

	mu                 sync.Mutex
	sessionStarts      map[string]time.Time
	firstDataObserved  map[string]struct{}
	timeToOpenResult   []time.Duration
	sessionDurations   []time.Duration
	timeToFirstC2SData []time.Duration
	timeToFirstS2CData []time.Duration
}

type StatsSnapshot struct {
	ChunksSent          uint64
	ChunksReceived      uint64
	BytesSent           uint64
	BytesReceived       uint64
	SealedBytesSent     uint64
	SealedBytesReceived uint64
	PutObjects          uint64
	GetObjects          uint64
	HeadObjects         uint64
	ListObjects         uint64
	DeleteObjects       uint64
	ActiveSessions      int64
	SessionsStarted     uint64
	SessionsOpened      uint64
	SessionsRejected    uint64
	SessionsCompleted   uint64
	SessionsFailed      uint64
	TimeToOpenResult    []time.Duration
	SessionDurations    []time.Duration
	TimeToFirstC2SData  []time.Duration
	TimeToFirstS2CData  []time.Duration
}

func (s *Stats) Snapshot() StatsSnapshot {
	if s == nil {
		return StatsSnapshot{}
	}
	s.mu.Lock()
	timeToOpenResult := append([]time.Duration(nil), s.timeToOpenResult...)
	sessionDurations := append([]time.Duration(nil), s.sessionDurations...)
	timeToFirstC2SData := append([]time.Duration(nil), s.timeToFirstC2SData...)
	timeToFirstS2CData := append([]time.Duration(nil), s.timeToFirstS2CData...)
	s.mu.Unlock()
	return StatsSnapshot{
		ChunksSent:          s.chunksSent.Load(),
		ChunksReceived:      s.chunksReceived.Load(),
		BytesSent:           s.bytesSent.Load(),
		BytesReceived:       s.bytesReceived.Load(),
		SealedBytesSent:     s.sealedSent.Load(),
		SealedBytesReceived: s.sealedReceived.Load(),
		PutObjects:          s.putObjects.Load(),
		GetObjects:          s.getObjects.Load(),
		HeadObjects:         s.headObjects.Load(),
		ListObjects:         s.listObjects.Load(),
		DeleteObjects:       s.deleteObjects.Load(),
		ActiveSessions:      s.activeSessions.Load(),
		SessionsStarted:     s.started.Load(),
		SessionsOpened:      s.opened.Load(),
		SessionsRejected:    s.rejected.Load(),
		SessionsCompleted:   s.completed.Load(),
		SessionsFailed:      s.failed.Load(),
		TimeToOpenResult:    timeToOpenResult,
		SessionDurations:    sessionDurations,
		TimeToFirstC2SData:  timeToFirstC2SData,
		TimeToFirstS2CData:  timeToFirstS2CData,
	}
}

func (s *Stats) incChunksSent(plainBytes, sealedBytes int) {
	if s != nil {
		s.chunksSent.Add(1)
		s.bytesSent.Add(uint64(plainBytes))
		s.sealedSent.Add(uint64(sealedBytes))
	}
}

func (s *Stats) incChunksReceived(plainBytes, sealedBytes int) {
	if s != nil {
		s.chunksReceived.Add(1)
		s.bytesReceived.Add(uint64(plainBytes))
		s.sealedReceived.Add(uint64(sealedBytes))
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

func (s *Stats) startSession(sessionID string, at time.Time) {
	if s == nil {
		return
	}
	s.started.Add(1)
	s.mu.Lock()
	if s.sessionStarts == nil {
		s.sessionStarts = make(map[string]time.Time)
	}
	s.sessionStarts[sessionID] = at
	s.mu.Unlock()
}

func (s *Stats) openSession(d time.Duration) {
	if s == nil {
		return
	}
	s.opened.Add(1)
	s.mu.Lock()
	s.timeToOpenResult = append(s.timeToOpenResult, d)
	s.mu.Unlock()
}

func (s *Stats) rejectSession(sessionID string, d time.Duration) {
	if s == nil {
		return
	}
	s.rejected.Add(1)
	s.mu.Lock()
	s.timeToOpenResult = append(s.timeToOpenResult, d)
	delete(s.sessionStarts, sessionID)
	delete(s.firstDataObserved, firstDataKey(sessionID, DirectionC2S))
	delete(s.firstDataObserved, firstDataKey(sessionID, DirectionS2C))
	s.mu.Unlock()
}

func (s *Stats) finishSession(sessionID string, d time.Duration, err error) {
	if s == nil {
		return
	}
	if err == nil {
		s.completed.Add(1)
	} else {
		s.failed.Add(1)
	}
	s.mu.Lock()
	s.sessionDurations = append(s.sessionDurations, d)
	delete(s.sessionStarts, sessionID)
	delete(s.firstDataObserved, firstDataKey(sessionID, DirectionC2S))
	delete(s.firstDataObserved, firstDataKey(sessionID, DirectionS2C))
	s.mu.Unlock()
}

func (s *Stats) recordFirstData(sessionID, direction string, at time.Time) {
	if s == nil {
		return
	}
	k := firstDataKey(sessionID, direction)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.firstDataObserved[k]; ok {
		return
	}
	if s.firstDataObserved == nil {
		s.firstDataObserved = make(map[string]struct{})
	}
	start, ok := s.sessionStarts[sessionID]
	if !ok {
		return
	}
	s.firstDataObserved[k] = struct{}{}
	switch direction {
	case DirectionC2S:
		s.timeToFirstC2SData = append(s.timeToFirstC2SData, at.Sub(start))
	case DirectionS2C:
		s.timeToFirstS2CData = append(s.timeToFirstS2CData, at.Sub(start))
	}
}

func firstDataKey(sessionID, direction string) string {
	return sessionID + "\x00" + direction
}
