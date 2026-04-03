# Werka Himoyalangan PDF Arxitekturasi Rejasi

Bu hujjat Werka archive hisobotlari uchun himoyalangan PDF oqimini qanday
qurish kerakligini belgilaydi.

Maqsad:

- PDF hisobotni server tomonda generatsiya qilish
- foydalanuvchiga tayyor `read-only` ko'rinishdagi fayl berish
- PDF'ni o'zgartirishni maksimal darajada qiyinlashtirish
- hujjatning haqiqiyligini server orqali tekshirish imkonini berish
- mobile app'ga xavfsiz va sodda download oqimini berish

Bu reja `Archive` bo'limi uchun yozilgan, lekin keyin boshqa reportlarga ham
shu pattern qo'llanadi.

## 1. Muhim printsip

PDF hech qachon source of truth bo'lmaydi.

Source of truth:

- ERPNext
- mobile_server

PDF esa:

- export qilingan snapshot
- ko'rish va ulashish uchun tayyor hujjat

Demak:

- PDF'ni 100% o'zgartirib bo'lmaydigan qilish degan narsa yo'q
- eng to'g'ri himoya bu:
  - static PDF
  - watermark
  - report id
  - verify endpoint
  - server-side audit

## 2. Nima uchun himoyalangan PDF kerak

Bu dasturdagi hisobotlar:

- biznes hujjatga yaqin
- ichida operatsion ma'lumot bor
- foydalanuvchi ko'rishi mumkin, lekin o'zboshimchalik bilan tahrir qilmasligi kerak

Shuning uchun PDF:

- oddiy editable export bo'lmasligi kerak
- interaktiv field'lar bo'lmasligi kerak
- keyinroq ulashilganda ham manbasi tekshiriladigan bo'lishi kerak

## 3. Tahdid modeli

Quyidagi narsalarni hisobga olamiz:

1. foydalanuvchi PDF ichidagi textni oddiy viewer bilan tahrirlashga urinadi
2. foydalanuvchi PDF'ni boshqa dasturda qayta saqlashga urinadi
3. foydalanuvchi PDF skrinshotini olib boshqacha hujjat qilib ko'rsatadi
4. boshqa odam PDF faylni olib haqiqiymi yoki yo'qmi bilolmaydi

Muhim haqiqat:

- 1 va 2 ni qiyinlashtirish mumkin
- 3 ni to'liq oldini olib bo'lmaydi
- 4 ni `verify` mexanizmi bilan hal qilsa bo'ladi

## 4. Himoya qatlamlari

To'g'ri arxitektura bir qatlamga emas, bir nechta qatlamga tayanadi.

### 4.1. Static PDF

PDF boshidan static bo'ladi.

Bu degani:

- form field yo'q
- editable annotation yo'q
- embedded JS yo'q
- user-editable metadata logikasi yo'q

Bu birinchi va eng muhim himoya.

### 4.2. Flatten

Agar generator ichida tasodifan interaktiv layer paydo bo'lsa, final chiqishda
flatten qilinadi.

Maqsad:

- hamma ko'rinadigan elementlar final content stream ichiga singib ketishi
- viewer ichida form yoki editable layer qolmasligi

Amalda:

- agar boshidan pure static generator ishlatilsa, flatten deyarli tabiiy ravishda
  bajarilgan bo'ladi
- lekin biz arxitektura talabida baribir `flattened output` siyosatini
  yozib qo'yamiz

### 4.3. Watermark

Har bir PDF'ga watermark qo'yiladi.

Kamida:

- `Accord Archive`
- generatsiya vaqti
- role
- user ref yoki display name

Ixtiyoriy:

- diagonal background watermark
- footer watermark

Watermark vazifasi:

- leak bo'lsa manbani izlash oson bo'ladi
- soxtalashtirish qiymati pasayadi

### 4.4. Report ID

Har bir PDF uchun yagona `report_id` yaratiladi.

Masalan:

- `WAR-SENT-20260403-160530-AB12`

Bu `report_id`:

- PDF header'da bo'ladi
- footer'da bo'ladi
- server audit log'da saqlanadi
- verify endpoint orqali tekshiriladi

### 4.5. Verification code yoki hash

PDF ichiga verify qilish uchun kod kiritiladi.

Variantlar:

- signed hash
- short verification code
- HMAC asosidagi token

Tavsiya:

- server secret bilan hisoblangan verification token
- foydalanuvchi ko'radigan va tekshiradigan format

Masalan:

- `verify_code: 7M4X-9Q2L-KD1P`

### 4.6. QR verification

PDF ichida QR bo'ladi.

QR quyidagiga olib boradi:

- `https://core.wspace.sbs/v1/mobile/reports/verify?id=...&code=...`

QR vazifasi:

- hujjatni tez tekshirish
- print holatda ham source'ni tasdiqlash

### 4.7. Digital signature

Bu kuchli, lekin keyingi bosqichdagi himoya.

Digital signature vazifasi:

- PDF keyin o'zgartirilsa signature buziladi
- hujjatning server tomonidan yaratilgani tasdiqlanadi

Bu juda foydali, lekin implementation murakkabroq.

Shuning uchun:

- Phase 1 da majburiy emas
- Phase 2 yoki 3 da qo'shiladi

### 4.8. Permissions flag

PDF metadata ichida:

- editing restricted
- copying restricted
- printing restricted

flag'lar bo'lishi mumkin.

Lekin bularni biz asosiy himoya deb hisoblamaymiz.

Sabab:

- ko'p viewerlar bu cheklovni aylanib o'tishi mumkin

Shuning uchun:

- qo'shimcha qatlam sifatida boradi
- lekin static + watermark + verify o'rnini bosmaydi

## 5. Tavsiya qilinadigan minimal kuchli kombinatsiya

Birinchi foydali va amaliy kombinatsiya:

1. static PDF
2. flattened output
3. watermark
4. report_id
5. verify_code
6. QR verify

Bu kombinatsiya kuchli va real ishlaydigan himoya beradi.

## 6. Server va mobile roli

### 6.1. Mobile app vazifasi

Mobile app:

- period va modulni tanlaydi
- serverdan PDF so'raydi
- PDF faylni qabul qiladi
- qurilmaga saqlaydi
- foydalanuvchiga ochib beradi

Mobile app hech qachon PDF'ni o'zi generatsiya qilmaydi.

### 6.2. Mobile server vazifasi

Mobile server:

- archive dataset'ni DB reader orqali yig'adi
- summary hisoblaydi
- PDF layout'ni generatsiya qiladi
- himoya qatlamlarini qo'shadi
- verify ma'lumotini saqlaydi
- PDF faylni mobile app'ga qaytaradi

## 7. Tavsiya qilinadigan server endpointlar

List endpoint alohida qoladi:

- `GET /v1/mobile/werka/archive?kind=sent&period=daily`

PDF endpoint alohida bo'ladi:

- `GET /v1/mobile/werka/archive/pdf?kind=sent&period=daily`

Verify endpoint:

- `GET /v1/mobile/werka/archive/pdf/verify?id=...&code=...`

Ixtiyoriy keyingi bosqich:

- `GET /v1/mobile/werka/archive/pdf/meta?id=...`

## 8. Tavsiya qilinadigan PDF response

Server response:

- `Content-Type: application/pdf`
- `Content-Disposition: attachment; filename="werka-sent-daily-2026-04-03.pdf"`

Mobile app file sifatida oladi.

## 9. Report metadata modeli

PDF generatsiya qilinganda server ichida alohida report metadata yozuvi
saqlanishi kerak.

Minimal metadata:

- `report_id`
- `kind`
- `period`
- `from`
- `to`
- `generated_at`
- `generated_by_role`
- `generated_by_ref`
- `generated_by_name`
- `verify_code`
- `dataset_hash`
- `record_count`

Saqlash joyi birinchi bosqichda sodda bo'lishi mumkin:

- JSON store
- yoki local lightweight data file

Masalan:

- `data/report_exports.json`

Bu metadata source of truth emas, audit/verify uchun ishlatiladi.

## 10. Dataset hash

Har bir PDF'ga dataset hash bog'lanadi.

Hash quyidagidan olinadi:

- report kind
- period
- from/to
- record ids
- qty
- status
- created_label

Bu hash:

- verify paytida ko'rsatiladi
- audit uchun kerak bo'ladi
- soxtalashtirilgan reportni aniqlashga yordam beradi

## 11. PDF ichidagi qat'iy tarkib

### 11.1. Header

- `Accord`
- report nomi
- period
- vaqt oralig'i
- generatsiya vaqti
- user info
- `report_id`

### 11.2. Summary

- yozuvlar soni
- UOM bo'yicha total

Muhim:

- har xil UOM bir-biriga qo'shilmaydi
- `Kg` alohida
- `Nos` alohida

### 11.3. Jadval

Har satr:

- sana
- document id
- counterparty
- item code
- item name
- qty
- uom
- status

### 11.4. Footer

- `report_id`
- `verify_code`
- `generated by Accord`
- QR verify

## 12. PDF generatsiya usuli

Tavsiya:

- server tomonda deterministic generator ishlatish
- interaktiv elementlarsiz PDF yaratish

Yo'llar:

1. HTML -> PDF
2. pure Go PDF generator

Mening tavsiyam:

- Phase 1 uchun sodda va deterministic usul
- layout stabil bo'lsin
- flatten natijasi kontrol qilinsin

## 13. Verify endpoint semantikasi

`verify` endpoint quyidagini qaytarishi kerak:

- report topildimi
- verify code mosmi
- kim generatsiya qilgan
- qachon generatsiya qilingan
- qaysi period uchun
- dataset hash
- status:
  - `valid`
  - `invalid`
  - `not_found`

Bu endpoint JSON qaytarsa ham bo'ladi.

Keyin xohlasak:

- web verify sahifa
- yoki simple HTML page

qo'shamiz.

## 14. Mobile app download oqimi

Mobile app'da:

1. user `Yuklab olish` bosadi
2. app serverdan PDF oladi
3. file local storage'ga yoziladi
4. userga success beriladi
5. kerak bo'lsa file ochiladi

Birinchi bosqichda yetarli:

- documents/downloads papkaga yozish
- file path ko'rsatish

Keyingi bosqichda:

- share
- open-in
- recent downloads

qo'shiladi

## 15. Security bo'yicha muhim cheklov

PDF himoyalangan bo'lsa ham:

- odam screenshot qilishi mumkin
- textni qayta terishi mumkin
- boshqa fayl qilib chiqarishi mumkin

Shuning uchun haqiqiy himoya:

- static export
- watermark
- verify endpoint
- server audit

Shu 4 tasi bo'lmasa, `flatten`ning o'zi yetarli emas.

## 16. Tavsiya qilinadigan rollout

### Phase 1. Himoyalangan static PDF

- static PDF
- no form/no annotation
- report_id
- verify_code
- watermark
- QR

### Phase 2. Mobile save/open

- mobile download
- local save
- open/share

### Phase 3. Stronger verification

- dataset hash
- verify endpoint
- audit store

### Phase 4. Digital signature

- real PDF signature
- signature validation

## 17. Majburiy talablar

Bu talablar arxitekturada majburiy deb olinadi:

1. PDF serverda generatsiya qilinadi
2. PDF static bo'ladi
3. PDF flattened output sifatida beriladi
4. watermark bo'ladi
5. report_id bo'ladi
6. verify_code bo'ladi
7. QR verify bo'ladi
8. mobile app tayyor file'ni oladi, o'zi generate qilmaydi

## 18. Birinchi implementatsiya slice

Eng to'g'ri birinchi slice:

1. `Archive PDF` endpoint
2. `static + watermark + report_id + verify_code`
3. mobile download button
4. file save

Digital signature keyingi bosqichga qoladi.

## 19. Yakuniy xulosa

Ha, himoyalangan PDF tashlab beradigan arxitektura to'liq qurish mumkin.

Eng to'g'ri yo'l:

- server-side generation
- static PDF
- flatten
- watermark
- report verification
- mobile app delivery

Shu yondashuv bilan hujjat:

- ko'rish uchun qulay
- tahrirlash uchun noqulay
- tekshirish uchun qulay
- biznes nuqtayi nazardan ishonchli bo'ladi
