# Werka Compliance PDF Implementatsiya Bosqichlari

Bu hujjat quyidagi maqsadli profilni amalda qanday bosqichlarda qurishni
belgilaydi:

- `PDF/A-3`
- `static`
- `flatten`
- `watermark`
- `report_id + verify_code + QR verify`
- `PAdES / Adobe CDS` imzo
- `lock after signing`
- `encryption yo'q`

Bu reja arxitektura emas, balki implementatsiya yo'l xaritasi hisoblanadi.

## 1. Yakuniy maqsad

Yakuniy natijada Werka archive report PDF'i:

1. arxivlashga mos `PDF/A-3` bo'ladi
2. interaktiv emas, static bo'ladi
3. flatten bo'ladi
4. watermark bilan chiqadi
5. `report_id` va `verify_code` bilan chiqadi
6. QR orqali verify qilinadi
7. `PAdES` yoki `Adobe CDS` orqali imzolanadi
8. imzodan keyin lock qilinadi

## 2. Muhim cheklovlar

Bu profil uchun quyidagi cheklovlarni oldindan qabul qilamiz:

### 2.1. Encryption ishlatilmaydi

Sabab:

- `PDF/A` bilan encrypt qilingan PDF birga yurmaydi

Demak:

- compliance PDF uchun `AES-256` yo'q

### 2.2. Hozirgi oddiy PDF writer yetmaydi

Biz yozib qo'ygan sodda PDF writer:

- static fayl beradi
- lekin `PDF/A-3` va `PAdES` darajasiga yetmaydi

Shuning uchun keyingi bosqichda:

- kuchliroq PDF stack kerak bo'ladi

### 2.3. Adobe CDS uchun tashqi trust kerak

`Adobe CDS` yoki real trustli `PAdES` uchun:

- certificate
- trust chain
- signing identity

kerak bo'ladi.

Bu qismni faqat kod bilan hal qilib bo'lmaydi.

## 3. Ishni 3 qatlamga bo'lamiz

### Qatlam 1. Biz o'zimiz qiladigan texnik qism

- archive dataset yig'ish
- report summary
- static layout
- watermark
- `report_id`
- `verify_code`
- `QR verify`
- server-side verify endpoint
- mobile download/save/open

### Qatlam 2. Compliance PDF qismi

- `PDF/A-3` generator
- embedded fonts
- metadata
- ICC profile
- conformance validation

### Qatlam 3. Trust va signature qismi

- `PAdES`
- `Adobe CDS`
- certificate
- lock after signing

## 4. Bosqichma-bosqich implementatsiya

## Phase 1. Static Protected PDF

Bu bosqichda maqsad:

- amaliy ishlaydigan himoyalangan export
- hali `PDF/A-3` va `PAdES` yo'q

Bo'ladigan ishlar:

1. server archive PDF endpoint
2. static PDF
3. flatten output siyosati
4. watermark
5. `report_id`
6. `verify_code`
7. QR code
8. mobile app download/save

Kutiladigan natija:

- user PDF yuklab oladi
- fayl oddiy tahrir uchun noqulay bo'ladi
- verify qilish uchun kod bo'ladi

Izoh:

- bu bosqich compliance emas
- lekin amaliy himoya beradi

## Phase 2. Verify Infrastructure

Bu bosqichda maqsad:

- PDF haqiqiyligini server orqali tekshirish

Bo'ladigan ishlar:

1. `report_exports` metadata store
2. `report_id` bo'yicha lookup
3. `verify_code` bo'yicha tekshirish
4. QR endpoint
5. oddiy verify API
6. xohlasak keyin HTML verify page

Kutiladigan natija:

- PDF o'ziga qarab emas, serverga qarab tekshiriladi
- foydalanuvchi yoki auditor QR bosib hujjatni tasdiqlay oladi

## Phase 3. PDF/A-3 Compliance

Bu bosqichda maqsad:

- PDF'ni arxivlash standarti darajasiga ko'tarish

Bo'ladigan ishlar:

1. `PDF/A-3`ga mos generator stack tanlash
2. embedded font siyosati
3. XMP metadata
4. ICC profile
5. attachment policy kerak bo'lsa alohida qaror
6. output validation

Muhim:

- bu bosqichda encryption yo'q
- output validator bilan tekshirilishi kerak

Kutiladigan natija:

- compliance PDF tayyor bo'ladi

## Phase 4. PAdES Signature

Bu bosqichda maqsad:

- hujjatni signature bilan himoyalash

Bo'ladigan ishlar:

1. signing certificate manbasini tanlash
2. server-side signing flow
3. `PAdES` imzo qo'shish
4. signature validation
5. timestamp authority kerak bo'lsa ulash

Kutiladigan natija:

- PDF keyin o'zgarsa signature buziladi

## Phase 5. Adobe CDS / Trust

Bu bosqichda maqsad:

- Adobe Reader ichida trustli imzo ko'rinishi

Bo'ladigan ishlar:

1. Adobe CDS yoki mos trust provider tanlash
2. account / certificate provisioning
3. production signing credentials
4. Adobe trust bilan test

Kutiladigan natija:

- Acrobat ichida signature trustli ko'rinadi

## Phase 6. Lock After Signing

Bu bosqichda maqsad:

- imzodan keyin hujjatni certification lock rejimiga qo'yish

Bo'ladigan ishlar:

1. certification signature mode tanlash
2. allowed changes policy:
   - none
   - yoki comments only
3. final signed output test

Kutiladigan natija:

- imzodan keyin har qanday o'zgarish vizual buzilish sifatida ko'rinadi

## 5. Har bosqichda qaysi stack ishlatiladi

### Phase 1-2 uchun

Minimal stack yetadi:

- current server-side PDF generation
- QR generation kutubxonasi
- metadata store

### Phase 3 uchun

Bizga `PDF/A-3`ga mosroq generator kerak bo'ladi.

Tanlov variantlari:

1. Go ichida kuchliroq PDF library
2. HTML -> PDF + post-process
3. tashqi PDF service

Tavsiya:

- imkon qadar server ichida deterministic generator
- external service'dan qochish

### Phase 4-5 uchun

Bu yerda odatda:

- PKI
- certificate
- trust provider

kerak bo'ladi.

Bu bosqichni kod yozishdan oldin procurement/ops bilan kelishish kerak.

## 6. Nima birinchi qilinadi

Eng to'g'ri yo'l:

1. `Phase 1`
2. `Phase 2`
3. `Phase 3`
4. `Phase 4`
5. `Phase 5`
6. `Phase 6`

Sabab:

- foydalanuvchiga tez qiymat beramiz
- compliance va trust qismini keyin qatlamlaymiz
- riskni bir urishda emas, bosqichma-bosqich yopamiz

## 7. Hozirgi real tavsiya

Agar hozir darrov production qiymat kerak bo'lsa, eng to'g'ri practical target:

- `Phase 1 + Phase 2`

Bu kombinatsiya beradi:

- static
- flatten
- watermark
- `report_id`
- `verify_code`
- QR verify
- mobile download

Bu allaqachon juda foydali va kuchli.

Keyin compliance talab qattiqlashsa:

- `Phase 3-6`

qo'shiladi.

## 8. Yakuniy qaror

Siz aytgan final profilni qurish **mumkin**.

Lekin u bir bosqichli ish emas.

To'g'ri implementatsiya:

1. avval protected static PDF
2. keyin verify infrastructure
3. keyin `PDF/A-3`
4. keyin `PAdES`
5. keyin `Adobe CDS`
6. keyin `lock after signing`

Shu yo'l bilan:

- ishlaydigan natija tez chiqadi
- compliance bosqichlari tartibli quriladi
- arxitektura chalkashib ketmaydi
