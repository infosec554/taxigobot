# TaxiBot - Tola Flow Tushuntirish (Uzbekcha)

## 📋 Tizim Haqida Umumiy Ma'lumot

**TaxiBot** - bu Telegram'da ishlayotgan **3 ta bot**ning birgalikda ishlaydigan taksiy buyurtma berish tizimi.

Har bir bot **o'z roli va vazifasi** bilan ishlaydi:
1. **Client Bot** 👤 - Mijozlar (buyurtma beruvchilar)
2. **Driver Bot** 🚖 - Haydovchilar (taksi vositachilari)
3. **Admin Bot** 🛠️ - Administrator (tizimni boshqaruvchilar)

Hammasi **bitta PostgreSQL bazasiga** ulangan.

---

## 🚀 Asosiy Jarayon (Main Process)

### **Tizim Ishga Tushirish**

```
1. Server start → Config load
2. Logger (jurnal) initialize qilish
3. PostgreSQL bazasiga ulanish
4. 3 ta bot yaratish va startlash
5. Web server ishga tushirish (mini app uchun)
6. Barcha bot va server parallelda ishlashni boshlash
```

---

## 👤 CLIENT (MIJOZ) FLOW - TOLIQ TUSHUNTIRISH

### **1️⃣ Boshlang'ich Ro'yxatdan O'tish**

**Client /start tugmasini bosadi:**

```
Client: /start
    ↓
Bot: "Salom! Xush kelibsiz" dedi
    ↓
Bot bazani tekshirdi: Bu user oldin ro'yxatdan o'tganmi?
    
    ├─ YO'Q bo'lsa: Yangi user record yaratadi
    └─ HA bo'lsa: Eski ma'lumotlarni olib keladi
    
    ↓
Bot user statusini tekshiradi:
    
    ├─ pending (ro'yxatdan o'tmagan)
        → Bot: "Telefon raqamini ulashing" tugmasi bilan
        → Client: Telefon raqamini ulashadi
        
    ├─ blocked (bloklangan)
        → Bot: "Siz blok qildingiz" xabari
        
    ├─ active (faol)
        → Menu ko'rsatadi va ishlatishni boshlaydi
```

### **2️⃣ Buyurtma Berish Jarayoni**

Client "➕ Buyurtma berish" tugmasini bosganda:

```
Client: "➕ Buyurtma berish"
    ↓
Bot State o'zgaradi: awaiting_from
Bot: "Qayerdan olib ketishni opsiz?" dedi
    ↓
Client: "Fergona Stansiyasi" dedi
    ↓
Bot State o'zgaradi: awaiting_to
Bot: "Qayga ketishni opsiz?" dedi
    ↓
Client: "Margilan Markaziy" dedi
    ↓
Bot State o'zgaradi: awaiting_tariff
Bot: "Qaysi turini tanlaysiz?" dedi
    ├─ 🚕 Ekonom - 15,000 so'm
    ├─ 🚗 Komfort - 25,000 so'm
    └─ 🏎️ Premium - 40,000 so'm
    ↓
Client: "Ekonom" tanladi
    ↓
Bot State o'zgaradi: awaiting_passengers
Bot: "Nechta yo'lovchi?" dedi
    ↓
Client: "3 ta" dedi
    ↓
Bot State o'zgaradi: awaiting_datetime
Bot: "Qaysi vaqtda kelib olib ketish kerak? (Misol: Bugun 18:00)" dedi
    ↓
Client: "Bugun 18:00" dedi
    ↓
Bot State o'zgaradi: awaiting_confirm
Bot: Buyurtmani ko'rsatadi:
    ┌───────────────────────────┐
    │ 🚕 Buyurtma Tafsiloti     │
    │ Boshlang'ich: Fergona     │
    │ Manzil: Margilan          │
    │ Tarif: Ekonom             │
    │ Yo'lovchilar: 3 ta        │
    │ Vaqt: Bugun 18:00         │
    │ NARX: 50,000 so'm         │
    │ [✅ Tasdiqlash] [❌ Rad]   │
    └───────────────────────────┘
    ↓
Client: "✅ Tasdiqlash" tugmasini bosadi
    ↓
💾 Buyurtma bazaga saqlanadi:
    ├─ Buyurtma ID: #123
    ├─ Client ID: uzatuvchining ID'si
    ├─ Driver ID: null (hali haydovchi yo'q)
    ├─ Boshlang'ich joy ID: Fergona
    ├─ Tugatish joy ID: Margilan
    ├─ Tarif ID: Ekonom
    ├─ Narx: 50,000 som
    ├─ Yo'lovchilar: 3
    ├─ Vaqt: 2026-02-14 18:00
    ├─ Status: "active" (haydovchi kutilmoqda)
    └─ Yaratilgan vaqti
    ↓
🔔 Xabarilar yuboriladi:
    ├─ Admin: "🔔 YANGI BUYURTMA! #123 | 50,000 som | Fergona → Margilan"
    ├─ Client: "✅ Buyurtmangiz qabul qilindi! Haydovchi kutilmoqda..."
    └─ Barcha Haydovchilar (Ekonom + Fergona-Margilan maршruti bilan):
        "🔔 YANGI BUYURTMA! #123 | 50,000 som | Fergona → Margilan"
```

### **3️⃣ Client Buyurtmani Kuzatish**

Client "📋 Mening buyurtmalarim" tugmasini bosganda:

```
Bot: Barcha buyurtmalarni ko'rsatadi:

📋 Mening Buyurtmalarim:
├─ #123 Fergona → Margilan (🟡 FAOL - Haydovchi kutilmoqda)
├─ #122 Chust → Andijan (🟢 HAYDOVCHI TOPILDI - Alisher)
├─ #121 Quva → Fergona (🚖 HAYDOVCHI KELMOQDA)
└─ #120 Tashkent → Chimkent (✅ TAYYOR - Tugatildi)

[Tafsilotlarni ko'rish uchun tugmasini bosing]
```

**Buyurtma Status'larini Kuzatish:**

```
Status: "active" (FAOL)
    → "Haydovchi kutilmoqda"
    → Haydovchi topilisini kutadi
    
    ↓ (Haydovchi qabul qilganda)
    
Status: "accepted" (QABUL QILINDI)
    → Client xabari: "🚖 Haydovchi topildi!"
    → Haydovchining nomi, telefoni, mashina raqami
    
    ↓ (Haydovchi harakatlanadi)
    
Status: "on_way" (KELMOQDA)
    → Client xabari: "🚖 Haydovchi sizga kelmoqda!"
    
    ↓ (Haydovchi keldi)
    
Status: "arrived" (KELDI)
    → Client xabari: "🚖 Haydovchi joyingizga keldi!"
    
    ↓ (Yo'lovchi o'tardi, safarni boshladik)
    
Status: "in_progress" (SAFARNI BOSHLANDI)
    → Client xabari: "▶️ Safaringiz boshlandi!"
    
    ↓ (Safarni tugadi)
    
Status: "completed" (TAYYOR)
    → Client xabari: "🏁 Safaringiz tugatildi! Rahmat!"
    
Yoki:

Status: "cancelled" (BEKOR QILINDI)
    → Buyurtma rad qilindi yoki tugadi
```

---

## 🚖 DRIVER (HAYDOVCHI) FLOW - TOLIQ TUSHUNTIRISH

### **1️⃣ Ro'yxatdan O'tish va Tasdiqlanish**

```
Haydovchi: /start
    ↓
Bot: "Salom! Haydovchi sifatida ro'yxatdan o'tishni xohlaysiz?" dedi
    ↓
Haydovchi: Telefon raqamini ulashadi
    ↓
Bot Status: "pending_signup" (ro'yxatdan o'tish jarayoni)
    ↓
Bot: "Mashinangizning markasini kiriting" (Misol: "Toyota")
    ↓
Haydovchi: "Toyota" yozdi
    ↓
Bot: "Modeli nima?" (Misol: "Camry")
    ↓
Haydovchi: "Camry" yozdi
    ↓
Bot: "Plastinka raqami nima?" (Misol: "10A123AA")
    ↓
Haydovchi: "10A123AA" yozdi
    ↓
💾 Haydovchi ma'lumotlari bazaga saqlanadi:
    ├─ Mashina Markasi: Toyota
    ├─ Mashina Modeli: Camry
    ├─ Plastinka: 10A123AA
    └─ Haydovchi ID: Uning Telegram ID
    
    ↓
Bot Status o'zgaradi: "pending_review" (ADMIN TASDIQLASHINI KUTMOQDA)
    
    ↓
Admin xabari oladi: "🚖 Yangi haydovchi tekshiruvni kutmoqda"
    
    ↓
ADMIN QAROR BERADI:
    
    ├─ ✅ QABUL QILDI
    │   └─ Haydovchi Status: "active" (FAOL)
    │   └─ Haydovchi xabari: "✅ Siz tasdiqlandi! Buyurtma olishni boshlashingiz mumkin"
    │
    └─ ❌ RAD QILDI
        └─ Haydovchi Status: "pending_review" (o'chirmay qoladi)
        └─ Haydovchi urinishi mumkin
```

### **2️⃣ Haydovchining Tayyorgarlik Settings'i**

Admin qabul qilgandan keyin, haydovchi menu ko'radi:

```
🚖 Haydovchi Menyu:
├─ 📦 Faol Buyurtmalar (mavjud buyurtmalarni ko'rish)
├─ 📍 Mening Maршrutlarim (qaysi shaharlar)
├─ 🚕 Mening Tariflarim (qaysi tarif turini qabul qilish)
├─ Sana bo'yicha Qidirish
└─ 📋 Mening Buyurtmalarim (qabul qilgan buyurtmalar)
```

#### **Tariflar Tanlab Olish:**

```
Haydovchi: "🚕 Mening Tariflarim"
    ↓
Bot: Barcha tarif turlarini ko'rsatadi:
    🚕 Tarif Tanlov:
    ├─ 🔴 Ekonom (qabul qilmayapti)
    ├─ 🔴 Komfort (qabul qilmayapti)
    ├─ 🔴 Premium (qabul qilmayapti)
    
    [Qaysi tarifni tanlay, shunga buyurtma oladi]
    
    ↓
Haydovchi: "Ekonom" tarifi bosadi
    ↓
Bot: Tarifni yoqadi (✅ Ekonom)
    ↓
Haydovchi: "Komfort" ni ham yoqadi
    ↓
Bot: Endi ikkalasi yoqildi (✅ Ekonom, ✅ Komfort)
    ↓
Haydovchi: "✅ Tayyor" tugmasini bosadi
    ↓
Haydovchi endi bu tariflar buyurtmalari oladi
```

#### **Mарштрутlar Tanlab Olish:**

```
Haydovchi: "📍 Mening Maршrutlarim"
    ↓
Bot: Barcha shaharlarni ko'rsatadi:
    📍 Shahar Tanlovi:
    ├─ Fergona
    ├─ Margilan
    ├─ Quva
    ├─ Andijan
    └─ Bosh Shahar
    
    Haydovchi: Qaysi shaxarlarda ishlashni opsiz?
    ↓
Haydovchi: "Fergona" va "Margilan" tanlab oldi
    ↓
Bot: Endi bu haydovchi faqat Fergona → Margilan 
     yo'nalishlardan buyurtma oladi
```

### **3️⃣ Buyurtma Qabul Qilish**

```
Haydovchi: "📦 Faol Buyurtmalar"
    ↓
Bot: Mavjud buyurtmalarni ko'rsatadi:
    
    📦 Mavjud Buyurtmalar:
    
    1️⃣ #123
       📍 Fergona → Margilan
       💰 Ekonom - 50,000 som
       👥 3 ta yo'lovchi
       🕐 Bugun 18:00
       [✅ QABUL QILISH]
    
    2️⃣ #124
       📍 Quva → Andijan
       💰 Komfort - 75,000 som
       👥 2 ta yo'lovchi
       🕐 Bugun 17:30
       [✅ QABUL QILISH]
    
    (Faqat haydovchining tarifi va maršruti bo'yicha)
    
    ↓
Haydovchi: #123 qabul qilish tugmasini bosadi
    ↓
Bot: Tekshiradi - bu buyurtma hali faol?
    ├─ HA: Davom
    └─ YO'Q: "❌ Bu buyurtma boshqa haydovchi oldi"
    
    ↓
💾 Buyurtma ma'lumotlari yangilanadi:
    ├─ Driver ID: Haydovchining ID
    ├─ Status: "accepted" (QABUL QILINDI)
    └─ Vaqti: Hozirgi vaqt
    
    ↓
🔔 Xabarilar:
    ├─ HAYDOVCHI: "✅ Buyurtma #123 qabul qilindi"
    ├─ CLIENT: "🚖 Haydovchi topildi! Ahmed | +998-91-123-45-67 | Toyota Camry | 10A123AA"
    └─ ADMIN: "Buyurtma qabul qilindi"
```

### **4️⃣ Safarni Kuzatish va Statusni Yangilash**

Haydovchi "📋 Mening Buyurtmalarim"ni bosadi:

```
Bot: Haydovchining qabul qilgan buyurtmalarini ko'rsatadi:

📋 Mening Buyurtmalarim:
#123 Fergona → Margilan
👥 3 yo'lovchi | 💰 50,000 som
Harakat qilish tugmalarini ko'radi:

[➡️ KELMOQDA] [✅ KELDIM] [▶️ SAFARNI BOSHLASH] [🏁 TUGADI]
```

**Status Yangilash Jarayoni:**

```
1️⃣ HAYDOVCHI: "➡️ KELMOQDA" tugmasini bosadi
    ↓
    Status: "accepted" → "on_way"
    ↓
    CLIENT XABARI: "🚖 Haydovchi sizga kelmoqda!"
    ADMIN: Bilgilanadi
    
    ↓
    
2️⃣ HAYDOVCHI: "✅ KELDIM" tugmasini bosadi (joyga keldi)
    ↓
    Status: "on_way" → "arrived"
    ↓
    CLIENT XABARI: "🚖 Haydovchi joyingizga keldi!"
    
    ↓
    
3️⃣ HAYDOVCHI: "▶️ SAFARNI BOSHLASH" (yo'lovchi o'tdi, safarni boshladi)
    ↓
    Status: "arrived" → "in_progress"
    ↓
    CLIENT XABARI: "▶️ Safaringiz boshlandi!"
    
    ↓
    
4️⃣ HAYDOVCHI: "🏁 TUGADI" (manziliga yetti)
    ↓
    Status: "in_progress" → "completed"
    ↓
    CLIENT XABARI: "🏁 Safaringiz tugatildi! Rahmat saza kul!"
    ↓
    💰 Buyurtma tugalandi, haydovchi pul oladi
    ↓
    Buyurtma tarihga o'tdi, aktiv ro'yxatdan olib tashlandi
```

---

## 🛠️ ADMIN (ADMINISTRATOR) FLOW - TOLIQ TUSHUNTIRISH

### **1️⃣ Admin Panelga Kirish**

```
Administrator: /start
    ↓
Bot: "Admin ID tekshirilmoqda..."
    ├─ ID to'g'ri: ✅ Admin menu ko'rsatadi
    └─ ID xato: ❌ "Doston, siz admin emassiz" dedi
```

### **2️⃣ Admin Menyu**

```
🛠️ ADMIN PANELI:

├─ 👥 Foydalanuvchilar (roli o'zgartirish, blok qilish)
├─ 📦 Barcha Buyurtmalar (tarix ko'rish)
├─ 📊 Statistika (nechta buyurtma, pul, vs)
├─ 🚖 Tasdiq Kutayotgan Haydovchilar (yangi haydovchilarni tekshirish)
├─ 📦 Tasdiq Kutayotgan Buyurtmalar (buyurtmalarni tasdiqlash)
├─ ⚙️ Tariflar (yangi tarif qo'shish/o'chirish)
└─ 🗺️ Shaharlar (yangi shahar qo'shish/o'chirish)
```

### **3️⃣ Tariflar Boshqarish**

```
Admin: "⚙️ Tariflar"
    ↓
Bot: Variantlarni ko'rsatadi:
    ├─ ➕ YANGI TARIF QOSH
    ├─ 🗑️ TARIF O'CHIR
    └─ Barcha Tariflar: Ekonom, Komfort, Premium...
    
    ↓
Admin: "➕ YANGI TARIF QOSH"
    ↓
Bot: "Tarif nomini kiriting (Misol: Ekonom Plus)"
    ↓
Admin: "Ekonom Plus" yozdi
    ↓
Bot: "Asosiy narx nima? (Misol: 20000)"
    ↓
Admin: "20000" yozdi
    ↓
Bot: "Kilometre uchun narx? (Misol: 2000)"
    ↓
Admin: "2000" yozdi
    ↓
💾 Tarif bazaga saqlanadi:
    ├─ Nomi: Ekonom Plus
    ├─ Asosiy narx: 20,000 som
    ├─ Kilometre narx: 2,000 som/km
    └─ ID: Auto
    
    ↓
Bot: "✅ Tarif qo'shildi!"
```

### **4️⃣ Shaharlar Boshqarish**

```
Admin: "🗺️ Shaharlar"
    ↓
Bot: 
    ├─ ➕ YANGI SHAHAR QOSH
    ├─ 🗑️ SHAHAR O'CHIR
    └─ 🔍 SHAHAR QID'IR
    
    ↓
Admin: "➕ YANGI SHAHAR QOSH"
    ↓
Bot: "Shahar nomini kiriting"
    ↓
Admin: "Namangan" yozdi
    ↓
💾 Bazaga qo'shildi
    ↓
Bot: "✅ Namangan qo'shildi!"
```

### **5️⃣ Haydovchilarni Tekshirish (Approval)**

```
Admin: "🚖 Tasdiq Kutayotgan Haydovchilar"
    ↓
Bot: Barcha "pending_review" haydovchilarni ko'rsatadi:

🚖 TASDIQ KUTAYOTGAN HAYDOVCHILAR:

1️⃣ Ahmed
   📱 +998-91-123-45-67
   🚗 Toyota Camry
   🏷️ 10A123AA
   [✅ TASDIQLASH] [❌ RAD QILISH]

2️⃣ Salim
   📱 +998-93-456-78-90
   🚗 Chevrolet Nexia
   🏷️ 15A456BB
   [✅ TASDIQLASH] [❌ RAD QILISH]

    ↓
Admin: "✅ TASDIQLASH" tugmasini bosadi
    ↓
💾 Haydovchi Status: "pending_review" → "active"
    ↓
🔔 XABARILAR:
    ├─ HAYDOVCHI: "✅ Siz tasdiqlandi! Buyurtma olishni boshlashingiz mumkin"
    ├─ ADMIN: Ro'yxatdan olib tashlandi
```

### **6️⃣ Buyurtmalarni Tasdiqlash (Agar System Shoshqaloq)**

```
Admin: "📦 Tasdiq Kutayotgan Buyurtmalar"
    ↓
Bot: "pending" status'li buyurtmalarni ko'rsatadi:

📦 TASDIQ KUTAYOTGAN BUYURTMALAR:

1️⃣ #125
   Client: Dilshod
   Route: Andijan → Tashkent
   Narx: 150,000 som
   [✅ TASDIQLASH] [❌ RAD QILISH]

    ↓
Admin: "✅ TASDIQLASH" tugmasini bosadi
    ↓
💾 Buyurtma Status: "pending" → "active"
    ↓
🔔 Barcha haydovchilarga xabari: "🔔 YANGI BUYURTMA!"
```

---

## 🔄 Buyurtma Status'ları - TOLIQ OYIN

Buyurtmaning boshidan oxirigacha o'tgan yo'li:

```
┌──────────────┐
│   pending    │  (Admin tasdiqlashini kutmoqda - NEW)
└──────┬───────┘
       ↓
┌──────────────┐
│   active     │  (Faol - Haydovchi kutilmoqda)
└──────┬───────┘
       ↓
┌──────────────┐
│  accepted    │  (Haydovchi topildi!)
└──────┬───────┘
       ↓
┌──────────────┐
│   on_way     │  (Haydovchi kelmoqda)
└──────┬───────┘
       ↓
┌──────────────┐
│   arrived    │  (Haydovchi keldik!)
└──────┬───────┘
       ↓
┌──────────────┐
│ in_progress  │  (Safarni boshlandi)
└──────┬───────┘
       ↓
┌──────────────┐
│  completed   │  (Safarni tugadi) ✅
└──────────────┘

YOKI ISTALGAN JOYDA:
       ↓
┌──────────────┐
│ cancelled    │  (Bekor qilindi) ❌
└──────────────┘
```

---

## 💬 Xabarilar Sistema (NOTIFICATION SYSTEM)

### **Buyurtma Yaratilganda:**

```
CLIENT: "✅ Buyurtmangiz qabul qilindi!"
        "Haydovchi kutilmoqda. Raqamingizni ulang..."

ADMIN:  "🔔 YANGI BUYURTMA!"
        "#123 | 50,000 som | Fergona → Margilan"

HAYDOVCHILAR (mos tarif + maршrut bilan):
        "🔔 YANGI BUYURTMA!"
        "#123 | 50,000 som | Fergona → Margilan"
        "[✅ QABUL QILISH]"
```

### **Haydovchi Qabul Qilganda:**

```
CLIENT: "🚖 Haydovchi topildi!"
        "Ahmed | +998-91-123-45-67"
        "Toyota Camry | 10A123AA"

ADMIN:  "#123 Qabul qilindi (Ahmed)"

HAYDOVCHI: "✅ Buyurtma qabul qilindi!"
```

### **Status O'zgarishlari:**

```
HAYDOVCHI "KELMOQDA" bosganda:
CLIENT: "🚖 Haydovchi sizga kelmoqda!"

HAYDOVCHI "KELDIM" bosganda:
CLIENT: "🚖 Haydovchi joyingizga keldi!"

HAYDOVCHI "SAFARNI BOSHLASH" bosganda:
CLIENT: "▶️ Safaringiz boshlandi!"

HAYDOVCHI "TUGADI" bosganda:
CLIENT: "🏁 Safaringiz tugatildi! Rahmat!"
```

---

## 💾 Ma'lumot Bazası - DATA MODELS

### **BUYURTMA (Order)**
```
Buyurtma Cadvali:
├─ ID: Buyurtmaning unikal raqami (auto #123)
├─ ClientID: Kim buyurtma berdi
├─ DriverID: Qaysi haydovchi qabul qildi (bo'sh bo'lishi mumkin)
├─ Boshlang'ich Joy: Fergona, Quva, va boshqalar
├─ Tugatish Joy: Margilan, Andijan, va boshqalar
├─ Tarif: Ekonom, Komfort, Premium
├─ Narx: Pul (50,000 som)
├─ Yo'lovchilar: Nechta (3 ta)
├─ Vaqt: Qaysi vaqtda kelib olib ketish (2026-02-14 18:00)
├─ Status: active, accepted, on_way, arrived, in_progress, completed
└─ Yaratilgan: Qaysi vaqtda (2026-02-14 15:30)
```

### **FOYDALANUVCHI (User)**
```
Foydalanuvchi Cadvali:
├─ ID: Auto ID
├─ Telegram ID: Uning Telegram ID'si
├─ Username: @nomisiz
├─ Nomi, Familiyasi
├─ Telefon Raqami
├─ Rol: client (mijoz), driver (haydovchi), admin (administrator)
├─ Status: pending, active, pending_signup, pending_review, blocked
└─ Yaratilgan Vaqti
```

### **HAYDOVCHI PROFILI (Driver Profile)**
```
Haydovchi Profili Cadvali:
├─ ID: Auto
├─ Owner ID: Haydovchining User ID'si
├─ Mashina Markasi: Toyota, Chevrolet, va boshqalar
├─ Mashina Modeli: Camry, Nexia, va boshqalar
├─ Plastinka Raqami: 10A123AA
└─ Verifikatsiya Status: pending, approved, rejected
```

---

## 🔐 Xavfsizlik (Security)

### **Admin Tekshiruvi:**

```go
Agar admin ID shu emas:
    → "Doston, siz admin emassiz" dedi
    → Menu ko'rsatmadi
```

### **Foydalanuvchi Tekshiruvi:**

```go
Agar Client Bot'da driver roli bo'lsa:
    → "Siz driver uchun bot'dan foydalaning"
    → Menu ko'rsatmadi
    
Agar Driver Bot'da client roli bo'lsa:
    → "Siz mijoz uchun bot'dan foydalaning"
    → Menu ko'rsatmadi
```

### **Status Tekshiruvi:**

```go
Agar status = "blocked":
    → "Siz blok qildingiz"
    
Agar status = "pending":
    → "Telefon raqamini ulashing"
    
Agar status = "pending_review" (haydovchi uchun):
    → "Admin tasdiqlashini kutingyapti"
    → Buyurtma ola olmaydi
```

---

## 📊 Tizimning Ishchi Modeli

### **Har Foydalanuvchi O'z Session'i Bor**

Session = Uning hozirgi holati (saqlash joyida):

```
Session Data:
├─ User ID: Uning bazadagi ID
├─ State: Hozirgi bosmasi (awaiting_from, awaiting_to, va boshqalar)
├─ Vaqti: Oxirgi harakatning vaqti
├─ Temp (vaqtinchalik) Ma'lumotlar: Buyurtma ma'lumotlari
└─ Tarixhi
```

**Misal:**
- Client "➕ Buyurtma berish" bosadi → State: "awaiting_from"
- "Fergona" yozadi → State: "awaiting_to" ga o'tadi
- Bu tarzda davom etadi...

---

## 🎯 Jami Ishchi Sxemasi (Big Picture)

```
TELEGRAM FOYDALANUVCHI
    ↓
Telegram Bot API
    ↓
BOT HANDLER (Habar qayta ishlash)
    ↓
SESSION (State tekshirish)
    ↓
DATABASE (Ma'lumot saqlash/olish)
    ↓
NOTIFICATION SYSTEM (Xabarilar yuborish)
    ↓
OTHER BOTS (Boshqa botlarga xabar yuborish)
    ↓
RESPONSE (Javob yuborish)
```

---

## ✨ TUGRI QILIB AYTGANDA

**TaxiBot** - bu **3 ta bot** (Client, Driver, Admin) bir **PostgreSQL bazasiga** ulangan. 

Har bir bot:
- **O'z roli** bilan ishlaydi
- **O'z menyu** bilan ishlaydi
- **O'z handler'lari** bilan amal qiladi
- **Bir bazadan** ma'lumot olib keladi

Hammasi **realtime** ishlaydi. Client buyurtma beradi → Admin bilgilanadi → Haydovchilar xabar oladi → Haydovchi qabul qiladi → Client bilgilanadi → Safarni boshladi → Client bilgilanadi → Safarni tugadi.

**XULOSA**: To'liq avtomatik taksi dispatcher tizimi Telegram'da!

