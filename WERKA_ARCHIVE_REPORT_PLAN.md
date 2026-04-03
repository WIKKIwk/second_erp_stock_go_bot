# Werka Archive Va Hisobot Arxitekturasi Rejasi

Bu hujjat Werka role uchun yangi `Archive` bo'limini to'g'ri arxitektura
asosida qurish rejasini belgilaydi.

Asosiy maqsad:

- `Recent` o'rniga foydali va biznesga mos bo'lgan `Archive` bo'lim berish
- Werka ishlarini `Qabul qilingan / Jo'natilgan / Qaytarilgan` bo'yicha ko'rsatish
- `Kunlik / Oylik / Yillik` kesimlarda tez yuklanadigan list va hisobot berish
- PDF hisobotni server tomonda generatsiya qilib mobile app'ga tayyor file
  sifatida berish
- ERPNext'ni source of truth qilib saqlash
- hozirgi `reader` fast-path kabi DB read orqali yuqori performance olish

## 1. Nega bu bo'lim kerak

`Recent` bo'lim Werka uchun amaliy jihatdan kuchli foyda bermadi.

Sabablar:

- oldingi action'ni topguncha qo'lda `+` bosib item tanlash tezroq
- `Recent` semantics Werka workflow'ga toza tushmadi
- `repeat` o'rniga `archive + report` ko'proq biznes qiymat beradi

Shuning uchun yangi `Archive` bo'limning vazifasi:

- tarixiy ishlarni toifalash
- period bo'yicha ko'rsatish
- PDF hisobot berish

## 2. Asosiy UX modeli

Werka dock'dagi `Archive` bo'limiga kirilganda foydalanuvchi 3 ta asosiy
modulni ko'radi:

1. `Qabul qilingan`
2. `Jo'natilgan`
3. `Qaytarilgan`

Har bir modulga kirilganda 3 ta period moduli chiqadi:

1. `Kunlik`
2. `Oylik`
3. `Yillik`

Masalan:

- Werka `Archive`
- `Jo'natilgan`
- `Kunlik`

shunda shu kesimdagi list ochiladi va tepada `Yuklab olish` tugmasi chiqadi.

`Yuklab olish` bosilganda:

- server PDF tayyorlaydi
- mobile app tayyor file'ni yuklab oladi
- file foydalanuvchi qurilmasida saqlanadi yoki ochiladi

## 3. Biznes semantikasi

Archive bo'limdagi har bir modulning ma'nosi oldindan qat'iy bo'lishi kerak.

### 3.1. Qabul qilingan

Bu Werka supplierdan qabul qilgan oqim.

Bu modulga quyidagilar kiradi:

- supplierdan kelgan va Werka tomondan qabul bo'lgan receiptlar
- kerak bo'lsa qisman qabul qilinganlar ham shu yerga kiradi

Bu yerda asosiy source:

- Purchase Receipt oqimi

### 3.2. Jo'natilgan

Bu Werka customerga jo'natgan oqim.

Bu modulga quyidagilar kiradi:

- Werka yaratgan Delivery Note'lar
- pending ham, accepted ham, partial ham bo'lishi mumkin
- lekin archive listda filter turi `jo'natish` bo'ladi

Bu yerda asosiy source:

- Delivery Note oqimi

### 3.3. Qaytarilgan

Bu qaytish yoki rad etish bilan bog'liq oqim.

Bu modulga quyidagilar kiradi:

- customer reject qilgan jo'natmalar
- customer partial qilgan jo'natmalar
- cancel bo'lgan oqimlar
- kerak bo'lsa supplier qaytarish bilan bog'liq yozuvlar keyingi bosqichda
  qo'shiladi

Bu yerda source:

- Delivery Note result state'lari
- kerak bo'lsa Purchase Receipt return state'lari

## 4. Period semantikasi

Period hisoblash server tomonda qat'iy qoidalarga ko'ra bo'lishi kerak.

### 4.1. Kunlik

- bugungi kun 00:00 dan hozirgacha

### 4.2. Oylik

- joriy oyning 1-kuni 00:00 dan hozirgacha

### 4.3. Yillik

Yillik uchun quyidagi qoida bo'ladi:

- odatiy holatda `now - 1 year` dan hozirgacha
- agar tizimda Werka tarixiy ma'lumoti 1 yildan kam bo'lsa:
  - birinchi mavjud Werka event sanasidan hozirgacha

Demak:

- app 1 yil ishlamagan bo'lsa ham bo'sh hisobot chiqmaydi
- mavjud butun tarix olinadi

## 5. Source Of Truth printsipi

ERPNext source of truth bo'lib qoladi.

Biz alohida archive database yaratmaymiz.

To'g'ri yo'l:

- `mobile_server` ERPNext DB va ERPNext document/state'lardan derive qiladi
- `mobile_app` faqat tayyor list va tayyor PDF file oladi

Bu yondashuvning foydasi:

- duplicat data yo'q
- business logic bitta joyda qoladi
- report va list bir xil source'dan chiqadi

## 6. Performance strategiyasi

Ha, bu bo'limda ham hozirgi kabi `DB reader fast-path` ishlatish kerak.

Sabab:

- archive listlar ko'p yozuvga ega bo'lishi mumkin
- report generatsiyasi uchun katta date range bo'yicha data kerak bo'ladi
- ERP API bilan buni har safar yig'ish sekin bo'ladi

Shuning uchun:

- list endpointlar DB reader orqali o'qiydi
- PDF endpointlar ham shu reader orqali dataset oladi
- fallback path zarur bo'lsa ERP API bo'ladi, lekin primary yo'l DB read bo'ladi

## 7. Tavsiya qilinadigan server endpointlar

Minimal va toza ko'rinish:

- `GET /v1/mobile/werka/archive?kind=received&period=daily`
- `GET /v1/mobile/werka/archive?kind=received&period=monthly`
- `GET /v1/mobile/werka/archive?kind=received&period=yearly`
- `GET /v1/mobile/werka/archive?kind=sent&period=daily`
- `GET /v1/mobile/werka/archive?kind=returned&period=daily`

PDF uchun:

- `GET /v1/mobile/werka/archive/pdf?kind=received&period=daily`
- `GET /v1/mobile/werka/archive/pdf?kind=sent&period=monthly`
- `GET /v1/mobile/werka/archive/pdf?kind=returned&period=yearly`

Yoki yanada canonical variant:

- `GET /v1/mobile/werka/archive?kind=sent&from=...&to=...`
- `GET /v1/mobile/werka/archive/pdf?kind=sent&from=...&to=...`

Men tavsiya qiladigan variant:

- app ichida `daily/monthly/yearly` ni `from/to` ga aylantirishni server o'zi
  qilsin
- ya'ni hozircha `period` bilan ishlash sodda va xavfsiz

## 8. Tavsiya qilinadigan response shakllari

### 8.1. List response

List uchun alohida report row DTO kerak.

Masalan:

```json
{
  "kind": "sent",
  "period": "daily",
  "from": "2026-04-03T00:00:00Z",
  "to": "2026-04-03T11:30:00Z",
  "summary": {
    "record_count": 27,
    "totals_by_uom": [
      { "uom": "Kg", "qty": 132.0 },
      { "uom": "Nos", "qty": 48.0 }
    ]
  },
  "items": [
    {
      "doc_id": "MAT-DN-2026-00066",
      "created_label": "2026-04-03 16:02:03.730575",
      "counterparty_ref": "ABCD Family",
      "counterparty_name": "ABCD Family",
      "item_code": "ABCD Family",
      "item_name": "ABCD Family",
      "qty": 1,
      "uom": "Kg",
      "status": "pending"
    }
  ]
}
```

### 8.2. PDF response

PDF endpoint:

- `Content-Type: application/pdf`
- `Content-Disposition: attachment; filename="werka-sent-daily-2026-04-03.pdf"`

Mobile app shu file'ni bevosita yuklab oladi.

## 9. PDF report ichidagi tarkib

PDF hisobotning formati qat'iy va tartibli bo'lishi kerak.

### 9.1. Header

- Hisobot nomi
  - masalan: `Werka Jo'natilgan Hisoboti`
- Period
  - `Kunlik / Oylik / Yillik`
- Vaqt oralig'i
- Generatsiya vaqti

### 9.2. Summary qismi

- yozuvlar soni
- UOM bo'yicha umumiy miqdorlar

Muhim:

- `Kg` va `Nos` kabi har xil UOM'larni bitta umumiy total qilib
  qo'shib yuborish mumkin emas
- total har doim `uom` bo'yicha alohida ko'rsatiladi

### 9.3. Jadval qismi

Har bir satrda:

- sana
- document ID
- customer yoki supplier nomi
- item code
- item name
- qty
- uom
- status

### 9.4. Footer

- jami sahifalar
- tizim nomi
- generatsiya timestamp

## 10. Mobile app arxitekturasi

Mobile tarafda ish asosan UI va file handling bo'ladi.

### 10.1. Yangi screen oqimi

1. Werka dock -> `Archive`
2. `Qabul qilingan / Jo'natilgan / Qaytarilgan`
3. `Kunlik / Oylik / Yillik`
4. list screen
5. `Yuklab olish`

### 10.2. Tavsiya qilinadigan screenlar

- `WerkaArchiveScreen`
- `WerkaArchivePeriodScreen`
- `WerkaArchiveListScreen`

### 10.3. Mobile store

App tarafda alohida store kerak:

- `WerkaArchiveStore`

Store quyidagilarni ushlaydi:

- `kind`
- `period`
- `loading`
- `error`
- `summary`
- `items`

### 10.4. PDF yuklab olish

Mobile app:

- serverdan PDF response oladi
- app documents directory yoki download directory'ga saqlaydi
- keyin foydalanuvchiga tayyor file sifatida beradi

Kerak bo'lsa keyin:

- `ochish`
- `share qilish`
- `download qilinganlar` listi

qo'shiladi

## 11. Server arxitekturasi

Asosiy og'ir logika serverda bo'lishi kerak.

### 11.1. Core layer

`mobile_server/internal/core/service.go` ichida yangi metodlar:

- `WerkaArchiveList(ctx, kind, period)`
- `WerkaArchivePDF(ctx, kind, period)`

### 11.2. Reader layer

`mobile_server/internal/erpdb/reader.go` ichida fast-path metodlar:

- `WerkaArchiveReceived(ctx, from, to)`
- `WerkaArchiveSent(ctx, from, to)`
- `WerkaArchiveReturned(ctx, from, to)`

Yoki bitta umumiy:

- `WerkaArchive(ctx, kind, from, to)`

### 11.3. Mobile API layer

`mobile_server/internal/mobileapi/server.go` ichida handlerlar:

- `handleWerkaArchive`
- `handleWerkaArchivePDF`

## 12. DB reader query strategiyasi

### 12.1. Received

Source:

- Purchase Receipt

Filter:

- Werka bilan bog'liq qabul yozuvlari
- `from/to` oralig'i

### 12.2. Sent

Source:

- Delivery Note

Filter:

- Werka customerga yuborgan jo'natmalar
- `accord_flow_state = submitted`
- `from/to`

### 12.3. Returned

Source:

- Delivery Note

Filter:

- `accepted / partial / rejected / cancelled` ichidan returned/rejected semantikasi
- `from/to`

Bu yerda returned semantics oldindan qat'iy yozilishi kerak.

Mening tavsiyam:

- `partial`
- `rejected`
- `cancelled`

birinchi bosqichda `returned` moduliga kirsin

## 13. Date range hisoblash qoidasi

Date range serverda hisoblanishi kerak.

Nega:

- timezone bitta joyda boshqariladi
- mobile app soddalashadi
- PDF va list bir xil oralig'ni ishlatadi

Server timezone:

- ERPNext ishlayotgan timezone yoki biznes timezone
- hozirgi kontekstda `Asia/Tashkent` mantiqiy

## 14. PDF generatsiya usuli

PDF serverda generatsiya qilinadi.

Bu uchun 3 ta yo'l bor:

1. HTML template -> PDF
2. pure Go PDF generator
3. existing report export path bo'lsa reuse

Mening tavsiyam:

- birinchi bosqichda pure Go generator yoki sodda HTML template
- tashqi og'ir dependency kiritmaslik

Muhim:

- PDF layout deterministic bo'lishi kerak
- mobile qurilma turiga bog'liq bo'lmasligi kerak

## 15. Xatoga chidamlilik

### 15.1. List endpoint

Agar PDF muvaffaqiyatsiz bo'lsa ham list ishlashi kerak.

### 15.2. PDF endpoint

PDF generatsiya yiqilsa:

- userga aniq xato qaytsin
- list endpoint bundan zarar ko'rmasin

### 15.3. Empty state

Agar davrda yozuv bo'lmasa:

- list bo'sh ko'rsatiladi
- PDF tugmasi yo o'chadi, yo `Bo'sh hisobot` siyosati bilan ishlaydi

Mening tavsiyam:

- item yo'q bo'lsa `Yuklab olish` disable bo'lsin

## 16. Tavsiya qilinadigan rollout

### Phase 1. Archive skeleton

- dock'dagi `Archive`
- 3 ta modul
- 3 ta period
- list screen skeleton

### Phase 2. Server list endpoint

- `received/sent/returned`
- `daily/monthly/yearly`
- DB reader fast-path

### Phase 3. Mobile list wiring

- archive list screen
- summary chiplar
- period bo'yicha list ko'rsatish

### Phase 4. PDF endpoint

- server PDF
- mobile download
- file save/open

### Phase 5. Polish

- UOM totals
- better table layout
- naming
- sharing

## 17. Birinchi implementatsiya slice uchun tavsiya

Eng to'g'ri birinchi slice:

1. `Archive` home
2. `Jo'natilgan`
3. `Kunlik / Oylik / Yillik`
4. faqat list endpoint
5. keyin PDF

Nega:

- jo'natilgan oqim hozir eng dolzarb
- notification va delivery note arxitekturasi endi tozalanib boryapti
- shu oqimni birinchi qilish eng ko'p biznes qiymat beradi

## 18. Muhim xulosa

Ha, bu feature uchun:

- server tomonda ham ishlash kerak
- mobile tomonda ham ishlash kerak

Lekin og'ir logika asosan serverda bo'lishi kerak.

To'g'ri yondashuv:

- ERPNext source of truth
- `mobile_server` orchestrator
- DB reader fast-path
- mobile app UI + download client

Shu yo'l bilan:

- list tez chiqadi
- PDF bir xil chiqadi
- bug ehtimoli kamayadi
- keyin archive'ni bosqichma-bosqich kengaytirish oson bo'ladi
