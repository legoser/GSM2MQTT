package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/sms"
	"github.com/legoser/gsm2mqtt/internal/sms/pdu"
)

// SetStorageManager sets the modem storage controller for reading and deleting stored SMS.
func (s *SMSService) SetStorageManager(mgr modem.StorageManager) {
	s.storageMgr = mgr
}

// SyncStoredMessages reads all stored SMS messages from the specified storages (e.g. "SM", "ME"),
// dispatches them through the assembler, and deletes them from the modem memory.
func (s *SMSService) SyncStoredMessages(ctx context.Context, storageNames ...string) (int, error) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	if s.storageMgr == nil {
		return 0, nil
	}
	if len(storageNames) == 0 {
		storageNames = []string{"SM", "ME"}
	}

	totalSynced := 0
	for _, storageName := range storageNames {
		select {
		case <-ctx.Done():
			return totalSynced, ctx.Err()
		default:
		}

		st, err := s.storageMgr.SelectStorage(storageName)
		if err != nil {
			slog.Debug("storage not available or empty", slog.String("storage", storageName), slog.Any("error", err))
			continue
		}

		if st != nil && st.Used == 0 {
			continue
		}

		msgs, err := s.storageMgr.ListMessages()
		if err != nil {
			slog.Warn("failed to list stored messages", slog.String("storage", storageName), slog.Any("error", err))
			continue
		}

		syncedInStorage := s.processAndPurgeMessages(msgs, storageName)
		totalSynced += syncedInStorage
	}

	return totalSynced, nil
}

func (s *SMSService) processAndPurgeMessages(msgs []modem.StoredMessage, storageName string) int {
	synced := 0
	for _, m := range msgs {
		if m.PDUHex == "" {
			continue
		}
		if m.Status == 2 || m.Status == 3 {
			_ = s.storageMgr.DeleteMessage(m.Index)
			continue
		}
		decoded, err := pdu.DecodeSMS(m.PDUHex)
		if err != nil {
			slog.Warn("failed to decode stored PDU", slog.Int("index", m.Index), slog.Any("error", err))
			_ = s.storageMgr.DeleteMessage(m.Index)
			continue
		}

		part := sms.IncomingPart{
			From:        decoded.From,
			Text:        decoded.Text,
			Timestamp:   decoded.Timestamp,
			IsMultipart: decoded.HasUDH,
			Reference:   decoded.Reference,
			PartNumber:  decoded.PartNumber,
			TotalParts:  decoded.TotalParts,
			Encoding:    string(decoded.Encoding),
		}

		if s.assembler != nil {
			assembled, complete := s.assembler.AddPart(part)
			if complete && s.onReceived != nil {
				s.onReceived(assembled)
			}
		}

		if err := s.storageMgr.DeleteMessage(m.Index); err != nil {
			slog.Warn("failed to delete processed SMS from storage",
				slog.Int("index", m.Index),
				slog.String("storage", storageName),
				slog.Any("error", err),
			)
		} else {
			slog.Debug("deleted processed stored SMS", slog.Int("index", m.Index), slog.String("storage", storageName))
		}
		synced++
	}
	return synced
}

// CheckStorageCapacity queries current storage utilization.
func (s *SMSService) CheckStorageCapacity() (*modem.StorageStatus, error) {
	if s.storageMgr == nil {
		return nil, fmt.Errorf("storage manager not configured")
	}
	return s.storageMgr.StorageCapacity()
}
