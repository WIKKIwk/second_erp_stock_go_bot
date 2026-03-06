package suplier

import flatbuffers "github.com/google/flatbuffers/go"

func encodeSuppliers(suppliers []Supplier) []byte {
	builder := flatbuffers.NewBuilder(256)

	offsets := make([]flatbuffers.UOffsetT, 0, len(suppliers))
	for _, supplier := range suppliers {
		name := builder.CreateString(supplier.Name)
		phone := builder.CreateString(supplier.Phone)
		supplierRecordStart(builder)
		supplierRecordAddName(builder, name)
		supplierRecordAddPhone(builder, phone)
		offsets = append(offsets, supplierRecordEnd(builder))
	}

	supplierBookStartSuppliersVector(builder, len(offsets))
	for i := len(offsets) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(offsets[i])
	}
	vector := builder.EndVector(len(offsets))

	supplierBookStart(builder)
	supplierBookAddSuppliers(builder, vector)
	root := supplierBookEnd(builder)
	builder.Finish(root)
	return builder.FinishedBytes()
}

func decodeSuppliers(buf []byte) []Supplier {
	if len(buf) == 0 {
		return nil
	}

	root := getRootAsSupplierBook(buf, 0)
	items := make([]Supplier, 0, root.SuppliersLength())
	record := &supplierRecordTable{}
	for i := 0; i < root.SuppliersLength(); i++ {
		if !root.Suppliers(record, i) {
			continue
		}
		items = append(items, Supplier{
			Name:  string(record.Name()),
			Phone: string(record.Phone()),
		})
	}
	return items
}
