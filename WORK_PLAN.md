- [ ] Customer login va Delivery Note confirmation oqimi
  Issue:
  - `mobile_server` repo: `#1`
  - https://github.com/WIKKIwk/mobile_server/issues/1

  Asosiy biznes oqim:
  1. `Werka` customerga mol jo'natadi
  2. backend `Delivery Note` yaratadi
  3. `Customer` mobil ilovaga login qiladi
  4. customer o'ziga tegishli jo'natmalarni ko'radi
  5. customer `Tasdiqlayman` yoki `Rad etaman` qiladi
  6. natija `Werka` va `Admin`ga qaytadi

  Backend:
  - [ ] `customer` role auth qo'shish
  - [ ] customer home/history/detail endpointlari
  - [ ] Delivery Note confirm/reject endpointlari
  - [ ] push qoidalarini customer roli bilan tozalash

  Mobile:
  - [ ] customer home screen
  - [ ] customer feed screen
  - [ ] customer detail screen
  - [ ] customer dock
  - [ ] confirm/reject UX

  Statuslar:
  - [ ] Kutilmoqda
  - [ ] Tasdiqlangan
  - [ ] Rad etilgan

  Muhim qoidalar:
  - [ ] customer faqat o'ziga tegishli Delivery Note larni ko'radi
  - [ ] sender emas, receiver birinchi signal oladi
  - [ ] response eventlari Werka/Admin ga qaytadi
  - [ ] oqim `supplier` bilan aralashmaydi

- [ ] Keyingi polishing
  - [ ] customer UX copy va visual tozalash
  - [ ] customer unread/badge qoidalarini alohida verify qilish
  - [ ] real Android device hard test
