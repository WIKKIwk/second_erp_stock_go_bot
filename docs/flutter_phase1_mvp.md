# Flutter Phase 1 MVP

## Goal

Android-first Flutter app uchun Phase 1 maqsadi:

- supplier va werka workflow'ni aniqlashtirish
- ekranlar ro'yxatini yopish
- navigation map ni belgilash
- minimal, tushunarli, operatsion UI tamoyillarini kelishib olish
- keyingi Phase 2 uchun aniq build scope tayyorlash

Bu phase ichida production API ulash yo'q. Bu UX va product structure phase.

## Product Direction

App turi:

- mobile-first operational app
- supplier va werka uchun juda sodda flow
- kam text, katta actionlar
- bir qo'l bilan ishlatishga mos
- sust internet sharoitida ham tushunarli holatlar

Platform plan:

- primary target: Android
- preview target: Linux desktop + Flutter Web
- future-ready: iPhone uchun codebase saqlanadi

## Primary Roles

### Supplier

Asosiy vazifa:

- o'ziga biriktirilgan mahsulotni tanlash
- jo'natilayotgan miqdorni kiritish
- jo'natishni tasdiqlash
- qabul qilinganini keyin ko'rish

Supplier uchun UX printsiplari:

- login tez bo'lsin
- item tanlash juda tez bo'lsin
- xato qilish qiyin bo'lsin
- har actiondan keyin aniq status ko'rinsin

### Werka

Asosiy vazifa:

- pending bildirishnomalarni ko'rish
- bitta dispatch'ni ochish
- real qabul qilingan miqdorni kiritish
- receipt tasdiqlash

Werka uchun UX printsiplari:

- pending list birinchi ekran bo'lsin
- supplier, item, qty bir qarashda ko'rinsin
- confirm jarayoni 2 ta bosqichdan oshmasin

## Core User Flows

### Flow 1: Supplier Login

1. User appni ochadi
2. Telefon yoki mavjud auth usuli bilan kiradi
3. Agar supplier account biriktirilgan bo'lsa Supplier Home ochiladi

### Flow 2: Supplier Dispatch

1. Supplier Home
2. `Jo'natish` tugmasi
3. Item picker
4. Qty entry
5. Confirm screen
6. Success state

Natija:

- draft Purchase Receipt yaratiladi
- werka notification queue'ga tushadi

### Flow 3: Werka Confirm

1. Werka Home yoki Pending screen
2. Pending dispatch list
3. Dispatch detail
4. Accepted qty entry
5. Confirm
6. Success state

Natija:

- Purchase Receipt submit bo'ladi
- supplierga confirmation holati qaytadi

### Flow 4: Supplier History

1. Supplier Home
2. Recent dispatches
3. Har birida status:
   - Draft
   - Kutilmoqda
   - Qabul qilindi

## MVP Screens

### 1. Splash / Boot

Purpose:

- app init
- auth/session check

UI:

- logo
- short loading text
- soft fade-in

### 2. Login

Purpose:

- user role aniqlash va kirish

Blocks:

- app title
- short subtitle
- primary login action
- fallback help text

State:

- loading
- error

### 3. Supplier Home

Purpose:

- supplier uchun asosiy kirish nuqtasi

Blocks:

- greeting
- primary CTA: `Jo'natish`
- recent dispatch list
- status chips

### 4. Supplier Item Picker

Purpose:

- supplierga tegishli itemlarni tanlash

Blocks:

- search field
- item cards
- item code
- item name
- UOM

### 5. Supplier Qty Entry

Purpose:

- jo'natilayotgan miqdorni kiritish

Blocks:

- selected item summary
- big numeric input
- UOM hint
- primary CTA

### 6. Supplier Confirm

Purpose:

- final approval

Blocks:

- supplier name
- item
- qty
- warehouse
- `Tasdiqlash`
- `Bekor qilish`

### 7. Supplier Success

Purpose:

- dispatch saqlandi holatini ko'rsatish

Blocks:

- success icon
- draft receipt id
- back to home button

### 8. Werka Home / Pending List

Purpose:

- pending bildirishnomalarni ko'rsatish

Blocks:

- count summary
- pending cards
- supplier
- item
- sent qty
- timestamp

### 9. Werka Dispatch Detail

Purpose:

- bitta dispatch'ni ko'rish va confirm qilish

Blocks:

- supplier block
- item block
- sent qty
- accepted qty input
- submit button

### 10. Werka Success

Purpose:

- qabul yakunlanganini ko'rsatish

Blocks:

- success state
- receipt id
- accepted qty
- back to list

## Navigation Map

Supplier:

- Splash -> Login -> Supplier Home
- Supplier Home -> Item Picker -> Qty Entry -> Confirm -> Success -> Supplier Home
- Supplier Home -> Dispatch History Detail

Werka:

- Splash -> Login -> Werka Home
- Werka Home -> Dispatch Detail -> Success -> Werka Home

## Information Architecture

### Supplier side

Main tabs MVP ichida shart emas.

Yetarli:

- Home
- History section same page

### Werka side

Main tabs MVP ichida shart emas.

Yetarli:

- Pending list
- Detail

## UI Style Direction

Overall direction:

- clean industrial minimalism
- very light background
- bold but sparse typography
- one strong accent color
- large cards
- rounded corners, but over-soft emas

Recommended visual language:

- background: warm light gray
- cards: white
- accent: deep green or deep blue
- warning: amber
- success: green
- danger: muted red

Typography:

- headline: expressive but readable
- body: neutral sans
- numbers: large, clear

## Motion

Motion principles:

- fast
- calm
- informative
- workflow'ni sekinlashtirmasligi kerak

Use:

- 150ms-250ms transitions
- fade + slide for page entry
- subtle scale for dialogs
- animated status chip updates
- pressed-state feedback

Avoid:

- long hero animations
- decorative motion
- bouncing interactions

## State Design

Har ekranda quyidagilar ko'zda tutiladi:

- empty state
- loading state
- error state
- success state

Examples:

- supplierda item yo'q
- pending dispatch yo'q
- internet yo'q
- ERP response slow

## Copy Tone

Matnlar:

- juda qisqa
- buyruqohang emas, aniq
- operatsion

Examples:

- `Mahsulot tanlang`
- `Miqdorni kiriting`
- `Jo'natishni tasdiqlaysizmi?`
- `Qabul qilingan miqdorni kiriting`

## Data Needed For MVP

Supplier dispatch card:

- item_code
- item_name
- uom
- sent_qty
- status
- created_at

Werka pending item:

- purchase_receipt_name
- supplier_name
- item_code
- item_name
- sent_qty
- uom
- created_at

## Phase 1 Output

Phase 1 tugagan deb hisoblash uchun quyidagilar tayyor bo'lishi kerak:

- supplier va werka flow freeze qilingan
- screen list freeze qilingan
- navigation map freeze qilingan
- visual direction kelishilgan
- Phase 2 build order aniq

## Phase 2 Build Order

1. Flutter shell
2. theme tokens
3. login screen
4. supplier home
5. dispatch flow
6. werka pending list
7. werka confirm flow
8. shared loading/error components
