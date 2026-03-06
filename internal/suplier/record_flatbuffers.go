package suplier

import flatbuffers "github.com/google/flatbuffers/go"

type supplierRecordTable struct {
	_tab flatbuffers.Table
}

func (rcv *supplierRecordTable) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *supplierRecordTable) Name() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *supplierRecordTable) Phone() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func supplierRecordStart(builder *flatbuffers.Builder) {
	builder.StartObject(2)
}

func supplierRecordAddName(builder *flatbuffers.Builder, name flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, name, 0)
}

func supplierRecordAddPhone(builder *flatbuffers.Builder, phone flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, phone, 0)
}

func supplierRecordEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
