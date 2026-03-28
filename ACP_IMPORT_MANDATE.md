# ACP Import Mandate

## Asosiy Vazifa

Bu hujjatning maqsadi bitta:

`ACP-Products.csv` importer ishidan chalg'imaslik.

Bu yerda asosiy vazifa:

1. `ACP-Products.csv` faylini o'qish.
2. `Agent` ustunini `Customer` sifatida talqin qilish.
3. `Nom` ustunini `Item name` va `Item code` sifatida ishlatish.
4. `Price` ustunini item narxi sifatida yozish.
5. `Barcode` ustunini item barcode sifatida yozish.
6. Har bir itemni aynan o'z agent-customeriga bog'lash.
7. `1 item = 1 customer` qoidasini buzmaslik.

## Nima Qilmaslik Kerak

Quyidagilarni qilish mumkin emas:

1. `ACP-Products.csv` vazifasidan chiqib ketish.
2. Foydalanuvchi so'ramagan `brnarsa.csv` bilan shug'ullanish.
3. Eski customerga tegishli itemni boshqa customerga foydalanuvchisiz ulash.
4. Overlapni yashirish.
5. "Qildim" deb aytib, verify qilmaslik.
6. Dry-run natijasini real import deb ko'rsatish.
7. Bitta itemga 2 ta customer yozib yuborish.
8. CSV formatini tekshirmasdan import qilish.
9. Foydalanuvchi aytgan asosiy vazifa o'rniga yon ishlarni qilish.

## ACP CSV Mapping

`ACP-Products.csv` ichida:

- `Agent` -> customer
- `Nom` -> item name
- `Price` -> item sof narxi
- `Barcode` -> barcode

Shuning uchun ACP importer quyidagicha ishlashi kerak:

1. Har bir qatorni o'qish.
2. `Agent` bo'yicha customer topish.
3. Customer topilmasa yaratish.
4. `Nom` bo'yicha item topish.
5. Item topilmasa yaratish.
6. `Price` ni itemga yozish.
7. `Barcode` bo'sh bo'lmasa, item barcode qo'shish.
8. Itemni customerga bog'lash.

## Xavfsizlik Qoidasi

Bu loyiha uchun qat'iy qoida:

`1 item = 1 customer`

Demak:

- agar item allaqachon boshqa customerga ulangan bo'lsa, ACP importer to'xtashi kerak
- userning alohida tasdig'isiz boshqa customer ustiga yozish mumkin emas
- overlap bo'lsa xato qaytishi kerak
- mavjud customer linkni yashirincha almashtirish mumkin emas

## Importdan Oldingi Tekshiruv

Har safar ACP importer ishlatishdan oldin:

1. CSV haqiqatan `ACP-Products.csv` ekanini tekshirish.
2. Header:
   - `Agent`
   - `Nom`
   - `Price`
   - `Barcode`
   borligini tekshirish.
3. `Price` vergul bilan yozilgan bo'lsa, floatga to'g'ri parse qilinishini tekshirish.
4. Target ERP credentials to'g'ri ekanini tekshirish.
5. Qaysi site/import qilinayotganini tekshirish.
6. Existing customer link overlap yo'qligini tekshirish.

## Importdan Keyingi Tekshiruv

Har safar ACP importer ishlatilgandan keyin quyidagilar tasdiqlanishi shart:

1. Nechta qator o'qildi.
2. Nechta unique item topildi.
3. Nechta customer yaratildi.
4. Nechta item yaratildi.
5. Nechta item allaqachon mavjud edi.
6. Nechta item customerga bog'landi.
7. Nechta barcode yozildi.
8. Nechta narx yozildi.

Keyin SQL/API verify qilish kerak:

1. `tabItem`
2. `tabItem Customer Detail`
3. kerak bo'lsa `tabItem Barcode`
4. itemdagi price maydoni

## Noto'g'ri Yo'nalishlar

Quyidagilar ACP vazifasi emas:

1. `brnarsa.csv` importlari
2. random customer assignlar
3. ERP transaction cleanup
4. mobile UI polish
5. unrelated bugfix
6. boshqa customer uchun itemlarni qo'lda ko'chirish

Bu ishlar faqat foydalanuvchi alohida buyurganda qilinadi.

## Hozirgi To'g'ri Fokus

Hozirgi to'g'ri fokus:

1. `ACP-Products.csv` importer
2. customer auto-resolve/create
3. item create/update
4. price update
5. barcode upsert
6. overlap-safe assign
7. real verify

## Ish Tartibi

ACP importer bilan ishlash tartibi:

1. kodni yozish
2. test yozish
3. `go test ./...`
4. dry-run
5. sample real run
6. to'liq run
7. SQL/API verify
8. commit

## Agar Chalg'ish Boshlansa

Agar agent ACP vazifasidan chalg'iyotgan bo'lsa:

1. darrov to'xtashi kerak
2. ACP vazifasiga qaytishi kerak
3. foydalanuvchidan so'ralmagan CSV bilan shug'ullanmasligi kerak
4. "hozirgi asosiy ish ACP importer" deb o'zini qayta yo'naltirishi kerak

## Amaliy Qoida

ACP vazifasi bo'yicha har safar o'ziga eslatma:

- faqat ACP
- faqat ACP
- faqat ACP
- faqat ACP
- faqat ACP

`ACP-Products.csv` vazifasi tugamaguncha boshqa yo'lga kirilmaydi.

## Yakuniy Maqsad

Yakuniy maqsad:

`ACP-Products.csv` ni bitta komandada, xavfsiz, overlap qilmaydigan, price va barcode bilan to'liq import qiladigan, qayta ishlatishga tayyor toolga aylantirish.
