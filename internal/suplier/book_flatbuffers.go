package suplier

import flatbuffers "github.com/google/flatbuffers/go"

type supplierBookTable struct {
	_tab flatbuffers.Table
}

func getRootAsSupplierBook(buf []byte, offset flatbuffers.UOffsetT) *supplierBookTable {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &supplierBookTable{}
	x.Init(buf, n+offset)
	return x
}

func (rcv *supplierBookTable) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *supplierBookTable) Suppliers(obj *supplierRecordTable, j int) bool {
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

func (rcv *supplierBookTable) SuppliersLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func supplierBookStart(builder *flatbuffers.Builder) {
	builder.StartObject(1)
}

func supplierBookAddSuppliers(builder *flatbuffers.Builder, suppliers flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, suppliers, 0)
}

func supplierBookStartSuppliersVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}

func supplierBookEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
