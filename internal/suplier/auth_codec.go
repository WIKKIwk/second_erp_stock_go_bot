package suplier

import (
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
)

func encodeSupplierAuths(items []SupplierAuth) []byte {
	builder := flatbuffers.NewBuilder(256)

	offsets := make([]flatbuffers.UOffsetT, 0, len(items))
	for _, item := range items {
		phone := builder.CreateString(item.Phone)
		passwordHash := builder.CreateString(item.PasswordHash)

		supplierAuthRecordStart(builder)
		supplierAuthRecordAddPhone(builder, phone)
		supplierAuthRecordAddPasswordHash(builder, passwordHash)
		supplierAuthRecordAddRegisteredAtUnix(builder, item.RegisteredAt.Unix())
		supplierAuthRecordAddLastLoginAtUnix(builder, item.LastLoginAt.Unix())
		supplierAuthRecordAddFailedAttempts(builder, int32(item.FailedAttempts))
		supplierAuthRecordAddLockedUntilUnix(builder, item.LockedUntil.Unix())
		supplierAuthRecordAddTelegramUserID(builder, item.TelegramUserID)
		offsets = append(offsets, supplierAuthRecordEnd(builder))
	}

	supplierAuthBookStartItemsVector(builder, len(offsets))
	for i := len(offsets) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(offsets[i])
	}
	vector := builder.EndVector(len(offsets))

	supplierAuthBookStart(builder)
	supplierAuthBookAddItems(builder, vector)
	root := supplierAuthBookEnd(builder)
	builder.Finish(root)
	return builder.FinishedBytes()
}

func decodeSupplierAuths(buf []byte) []SupplierAuth {
	if len(buf) == 0 {
		return nil
	}

	root := getRootAsSupplierAuthBook(buf, 0)
	items := make([]SupplierAuth, 0, root.ItemsLength())
	record := &supplierAuthRecordTable{}
	for i := 0; i < root.ItemsLength(); i++ {
		if !root.Items(record, i) {
			continue
		}

		item := SupplierAuth{
			Phone:          string(record.Phone()),
			PasswordHash:   string(record.PasswordHash()),
			FailedAttempts: int(record.FailedAttempts()),
			TelegramUserID: record.TelegramUserID(),
		}
		if unix := record.RegisteredAtUnix(); unix > 0 {
			item.RegisteredAt = time.Unix(unix, 0).UTC()
		}
		if unix := record.LastLoginAtUnix(); unix > 0 {
			item.LastLoginAt = time.Unix(unix, 0).UTC()
		}
		if unix := record.LockedUntilUnix(); unix > 0 {
			item.LockedUntil = time.Unix(unix, 0).UTC()
		}
		items = append(items, item)
	}
	return items
}
