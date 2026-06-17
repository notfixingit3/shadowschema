package spec

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"
)

const defaultSaveDebounce = 2 * time.Second

func saveDebounceInterval() time.Duration {
	if raw := os.Getenv("SHADOWSCHEMA_SAVE_DEBOUNCE_MS"); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return defaultSaveDebounce
}

func (s *SpecManager) scheduleSave() {
	s.saveTimerMu.Lock()
	defer s.saveTimerMu.Unlock()

	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.saveTimer = time.AfterFunc(saveDebounceInterval(), s.flushSave)
}

func (s *SpecManager) flushSave() {
	s.saveTimerMu.Lock()
	s.saveTimer = nil
	s.saveTimerMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.persistLocked()
}

// Flush persists the in-memory spec immediately. Safe to call during shutdown.
func (s *SpecManager) Flush() {
	s.saveTimerMu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
	s.saveTimerMu.Unlock()

	s.flushSave()
}

func (s *SpecManager) persistLocked() {
	data, err := json.Marshal(s.doc)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal spec for DB: %v", err)
		return
	}
	_, err = s.dbExec(`UPDATE sessions SET spec_json = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, string(data), s.SessionID)
	if err != nil {
		log.Printf("[ERROR] Failed to save state to DB: %v", err)
	}
}