# Professional Taxi Bot Implementation Plan (Dual-Bot System)

## 🔷 1. UMUMIY TIZIM QOIDALARI
### 1.1 Rollar
- **Admin**: Tizim egasi. `.env` dagi `ADMIN_ID` orqali aniqlanadi. Botga birinchi marta kirganda avtomatik `admin` roli biriktiriladi.
- **Driver**: Faqat Admin tomonidan tayinlanadi. **Driver/Admin** botidan foydalanadi.
- **Client**: Standart foydalanuvchi. **Mijoz** botidan foydalanadi.

### 1.2 User status
- `pending`: Telefon raqami yuborilmagan (yangi foydalanuvchi).
- `active`: Tizimdan to'liq foydalanish huquqiga ega.
- `blocked`: Admin tomonidan botdan chetlatilgan.

### 1.3 Order status
- `active`: Yangi, haydovchi kutayotgan zakaz.
- `taken`: Haydovchi qabul qilgan zakaz.
- `completed`: Muvaffaqiyatli bajarilgan zakaz.
- `cancelled`: Mijoz yoki Admin tomonidan bekor qilingan.

---

## 🛠 0. ADMIN INITIALIZATION (Xavfsizlik)
- Bot ishga tushganda `Config` orqali `ADMIN_ID` (Telegram User ID) yuklanadi.
- **Logic:** `if caller_id == ADMIN_ID then set user.role = 'admin', user.status = 'active'`.
- Bu tizimni qo'lda o'zgartirishsiz (Super Admin huquqi bilan) boshlash imkonini beradi.

---

## 🔵 2. CLIENT LOGIKASI (Mijoz Boti - Bot 1)
**🎯 Maqsad:** Foydalanuvchi tajribasini (UX) sodda va tezkor qilish.

**STEP C1 — Ro‘yxatdan o‘tish**
1. `/start` bosilganda:
   - Foydalanuvchi topilmasa: `role = client`, `status = pending`.
   - Telefon raqami so'raladi (`Share Contact` tugmasi orqali).
2. Raqam yuborilgach:
   - `status = active` ga o'zgaradi.
   - Xush kelibsiz xabari va Mijoz menyusi chiqadi.

**STEP C2 — Zakaz berish (State Machine)**
1. **➕ Zakaz berish** tugmasi bosiladi.
2. Bot: "📍 Qayerdan olasiz?" -> Client matn yuboradi.
3. Bot: "🏁 Qayerga borasiz?" -> Client matn yuboradi.
4. Bot: "🚕 Tarifni tanlang" -> Bazadagi `tariffs` jadvalidan ro'yxat (Inline).
5. Bot: "👥 Necha kishi?" -> Raqam kiritiladi.
6. Bot: "📅 Ketish vaqti?" -> Matn kiritiladi.
7. Bot: "💰 Hammasi to'g'rimi? [TASDIQLASH | BEKOR QILISH]".
8. **Natija:** Order yaratiladi (`status = active`).
9. **Notification:** **Driver/Admin botiga** barcha haydovchilarga xabar yuboriladi.

---

## 🟡 3. DRIVER/ADMIN LOGIKASI (Driver/Admin Boti - Bot 2)
"Admin haydovchi orqasida yashirinadi" — Ikkala rol bitta interfeyda, lekin huquqlar turlicha.

**STEP D1 — Kirish va Xavfsizlik**
- Ushbu botga faqat `role = 'driver'` yoki `role = 'admin'` bo'lganlar kira oladi.
- Boshqalar uchun: "🚫 Kirish taqiqlangan".

**STEP D2 — Haydovchi Flow**
1. **📦 Faol zakazlar** tugmasi bor.
2. Bosilganda: Faqat `active` zakazlar chiqadi.
3. **"Zakazni olish"** (Inline):
   - Tizim tekshiradi: `if order.status == 'active'`.
   - Muvaffaqiyatli: `order.status = 'taken'`, `order.driver_id = current_user_id`.
   - **Notification:** Mijozga zakaz olingani haqida xabar boradi.

**STEP D3 — Zakazni yakunlash**
1. **📋 Mening zakazlarim** -> Haydovchi o'zi olgan zakazni ko'radi.
2. **"✅ Yakunlash"**: `order.status = 'completed'`.
3. **Notification:** Mijozga rahmatnoma yuboriladi.

---


## 🔴 4. ADMIN LOGIKASI (Hidden Menu)
**🎯 Maqsad:** Haydovchi menyusi ichida qo'shimcha boshqaruv paneli.

**STEP A1 — Admin Menyu Strukturasi**
Agar foydalanuvchi `role == 'admin'` bo'lsa, unga quyidagi tugmalar ham qo'shiladi:
- `👥 Userlar`: Rollarni o'zgartirish (Clientni haydovchi qilish), bloklash.
- `📦 Jami zakazlar`: Barcha faol va bajarilgan zakazlar monitoringi.
- `⚙️ Tariflar`: Yangi tarif qo'shish yoki o'chirish.
- `🗺 Yo'nalishlar`: Manzillarni tahrirlash.
- `📊 Statistika`: Kunlik zakazlar va tushum.

---

## 🟣 5. STATUS VA NOTIFICATION QOIDALARI
- **Status Life Cycle:** `active` → `taken` → `completed`.
- **Atomic Operations:** Zakazni olishda "Double take" (ikki kishi olishi) oldi olinadi (Database locking/check).
- **Notification Events:** Faqat status o'zgarganda (Create, Taken, Completed, Cancelled).

---

## 🧠 YAKUNIY XULOSA
Ushbu reja asosida bot **2 ta alohida token** bilan ishlaydi. Mijozlar ochiq botdan foydalanadi, haydovchi va adminlar esa bitta yopiq interfeyda, lekin o'z huquqlari doirasida ishlaydi.
