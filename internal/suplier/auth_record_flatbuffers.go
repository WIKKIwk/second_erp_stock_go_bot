package suplier

import flatbuffers "github.com/google/flatbuffers/go"

type supplierAuthRecordTable struct {
	_tab flatbuffers.Table
}

func (rcv *supplierAuthRecordTable) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *supplierAuthRecordTable) Phone() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *supplierAuthRecordTable) PasswordHash() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *supplierAuthRecordTable) RegisteredAtUnix() int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		return rcv._tab.GetInt64(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *supplierAuthRecordTable) LastLoginAtUnix() int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		return rcv._tab.GetInt64(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *supplierAuthRecordTable) FailedAttempts() int32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		return rcv._tab.GetInt32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *supplierAuthRecordTable) LockedUntilUnix() int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(14))
	if o != 0 {
		return rcv._tab.GetInt64(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *supplierAuthRecordTable) TelegramUserID() int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(16))
	if o != 0 {
		return rcv._tab.GetInt64(o + rcv._tab.Pos)
	}
	return 0
}

func supplierAuthRecordStart(builder *flatbuffers.Builder) {
	builder.StartObject(7)
}

func supplierAuthRecordAddPhone(builder *flatbuffers.Builder, phone flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, phone, 0)
}

func supplierAuthRecordAddPasswordHash(builder *flatbuffers.Builder, passwordHash flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, passwordHash, 0)
}

func supplierAuthRecordAddRegisteredAtUnix(builder *flatbuffers.Builder, registeredAtUnix int64) {
	builder.PrependInt64Slot(2, registeredAtUnix, 0)
}

func supplierAuthRecordAddLastLoginAtUnix(builder *flatbuffers.Builder, lastLoginAtUnix int64) {
	builder.PrependInt64Slot(3, lastLoginAtUnix, 0)
}

func supplierAuthRecordAddFailedAttempts(builder *flatbuffers.Builder, failedAttempts int32) {
	builder.PrependInt32Slot(4, failedAttempts, 0)
}

func supplierAuthRecordAddLockedUntilUnix(builder *flatbuffers.Builder, lockedUntilUnix int64) {
	builder.PrependInt64Slot(5, lockedUntilUnix, 0)
}

func supplierAuthRecordAddTelegramUserID(builder *flatbuffers.Builder, telegramUserID int64) {
	builder.PrependInt64Slot(6, telegramUserID, 0)
}

func supplierAuthRecordEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
