
- [ ] Werka "Aytilmagan mol" oqimi
  Kelishilgan reja:
  1. `Werka` center tugma `+` bo'ladi.
  2. `Werka` yangi page ga o'tadi.
  3. `Aytilmagan mol` card bosiladi.
  4. `Supplier tanlang`.
  5. `Mahsulot + qty` kiriting.
  6. `Tasdiqlash` ekranida 2-step confirm.
  7. Backend `Purchase Receipt` draft ochadi, lekin uni oddiy supplier flow'dan alohida marker bilan belgilaydi (`werka_unannounced`).
  8. `Supplier`ga notification boradi: werka qayd etilmagan mahsulotni qabul qilgani haqida.
  9. `Supplier` detail ekranida 2 variant bo'ladi:
     - `Tasdiqlayman`
     - `Rad etaman`
  10. `Tasdiqlayman` bo'lsa draft submit qilinadi.
  11. `Rad etaman` bo'lsa draft submit bo'lmaydi, note/comment yoziladi va werka/admin ko'radi.
  Muhim qoidalar:
  - supplier final authority
  - approve/reject shart
  - oddiy werka confirm flow'dan alohida bo'lishi shart
  - history/notificationlarda bu receipt turi aniq ajralib turishi shart
