package suplier

import flatbuffers "github.com/google/flatbuffers/go"

type supplierAuthBookTable struct {
	_tab flatbuffers.Table
}

func getRootAsSupplierAuthBook(buf []byte, offset flatbuffers.UOffsetT) *supplierAuthBookTable {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &supplierAuthBookTable{}
	x.Init(buf, n+offset)
	return x
}

func (rcv *supplierAuthBookTable) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *supplierAuthBookTable) Items(obj *supplierAuthRecordTable, j int) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o == 0 {
		return false
	}
	x := rcv._tab.Vector(o)
	x += flatbuffers.UOffsetT(j) * 4
	x = rcv._tab.Indirect(x)
	obj.Init(rcv._tab.Bytes, x)
	return true
}

func (rcv *supplierAuthBookTable) ItemsLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func supplierAuthBookStart(builder *flatbuffers.Builder) {
	builder.StartObject(1)
}

func supplierAuthBookAddItems(builder *flatbuffers.Builder, items flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, items, 0)
}

func supplierAuthBookStartItemsVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}

func supplierAuthBookEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
