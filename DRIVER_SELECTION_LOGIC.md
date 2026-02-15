# DRIVER SELECTION LOGIC - 100 Ta Driverni 1 Driverni Tanlash

## ❓ SAVOL
Admin order approve qildi → 100 ta driver mavjud
**Qaysiga order berish kerak?**

---

## ✅ JAVOB: SMART MATCHING ALGORITHM

### **Sinaario: Order Details**
```
Order #123:
├─ From: Fergona (Latitude: 40.3828, Longitude: 71.7788)
├─ To: Margilan (Latitude: 40.4917, Longitude: 71.7314)
├─ Tariff: Economy
├─ Time: Today 18:00
└─ Budget: 50,000 som
```

---

## 🎯 SMART DRIVER SELECTION (4-STEP PROCESS)

### **STEP 1: Filter Matching Drivers**

```
100 TA DRIVER → FILTER
    ↓
1️⃣ STATUS CHECK
   ├─ status = "active" ONLY ✅
   └─ "pending_review" skip ❌
   
   After filter: 80 ta driver
    ↓
2️⃣ TARIFF CHECK
   ├─ Driver selected "Economy" tariff ✅
   └─ Driver selected only "Comfort" ❌
   
   After filter: 60 ta driver
    ↓
3️⃣ ROUTE CHECK
   ├─ Driver works "Fergona → Margilan" ✅
   ├─ Driver works "Quva → Andijan" ❌
   └─ Driver has NO route (all routes ok) ✅
   
   After filter: 50 ta driver
    ↓
4️⃣ AVAILABILITY CHECK
   ├─ Driver has < 3 active orders ✅
   └─ Driver has 5+ active orders ❌
   
   After filter: 40 ta driver MATCHED
```

---

### **STEP 2: Calculate Score for Each Matched Driver**

```
For each of 40 matched drivers:

SCORE = (Rating × 40%) + (Distance × 30%) + (Availability × 20%) + (Response Time × 10%)

Example Driver #1 (Alisher):
├─ Rating: 4.8/5 → Score: 4.8 × 0.4 = 1.92
├─ Distance: 2 km → Score: (5-2) × 0.3 = 0.9 points
├─ Availability: 1 order (5 max) → Score: (5-1)/5 × 0.2 = 0.16
├─ Response: 2 sec (avg 5) → Score: 2/5 × 0.1 = 0.04
└─ TOTAL: 1.92 + 0.9 + 0.16 + 0.04 = 3.02/5

Example Driver #2 (Otabek):
├─ Rating: 3.5/5 → 1.4
├─ Distance: 8 km → 0.6
├─ Availability: 3 orders → 0.08
├─ Response: 4 sec → 0.08
└─ TOTAL: 2.16/5

Example Driver #3 (Shukhrat):
├─ Rating: 4.2/5 → 1.68
├─ Distance: 1 km → 1.2
├─ Availability: 0 orders → 0.2
├─ Response: 1 sec → 0.1
└─ TOTAL: 3.18/5 ⭐ BEST!
```

---

### **STEP 3: Sort by Score**

```
ALL 40 MATCHED DRIVERS SORTED:

1. Shukhrat: 3.18 ⭐⭐⭐ WINNER
2. Alisher: 3.02
3. Dilshod: 2.98
4. Otabek: 2.16
5. Bobur: 2.10
... (35 more)
40. Karim: 0.95
```

---

### **STEP 4: Send Notification to TOP DRIVERS**

```
OPTION A: Send only to #1 (Best Driver)
├─ Shukhrat gets notification
├─ Very fair, best matching
└─ But what if Shukhrat busy? Order waits!

OPTION B: Send to TOP 3 (Racing)
├─ Shukhrat, Alisher, Dilshod get notification
├─ First to click accepts
├─ Fair + backup
└─ But too fast, not fair to others

OPTION C: Send to TOP 10 (Graduated)
├─ Top 10 drivers get notification
├─ If no response in 10 sec → Top 10-20
├─ Fair + responsive
└─ RECOMMENDED ✅

OPTION D: Smart Fallback
├─ Try Shukhrat (30 sec timeout)
├─ If no response → Try Alisher
├─ If no response → Try Dilshod
├─ Automatic escalation
└─ Best for critical orders
```

---

## 📊 DETAILED SCORING FORMULA

### **Rating Score (40%)**
```go
func ratingScore(driverRating float64) float64 {
    // Rating is 0-5 stars
    // Return as 0-2 points (40% of 5)
    return driverRating * (2.0 / 5.0)
}

Examples:
- 5.0 stars → 2.0 points
- 4.0 stars → 1.6 points
- 3.0 stars → 1.2 points
```

### **Distance Score (30%)**
```go
func distanceScore(driverLat, driverLon, orderLat, orderLon float64) float64 {
    // Calculate haversine distance
    distance := haversineDistance(driverLat, driverLon, orderLat, orderLon)
    
    // Closer = better
    // 0 km = 1.5 points (30% of 5)
    // 5 km = 0 points
    // Formula: max(0, 1.5 - (distance / 3.33))
    return max(0, 1.5 - (distance / 3.33))
}

Examples:
- 0 km (same location) → 1.5 points
- 2 km → 1.1 points
- 5 km → 0 points
- 10 km → negative (capped at 0)
```

### **Availability Score (20%)**
```go
func availabilityScore(activOrders int, maxOrders int) float64 {
    // More free = better
    // 0 active = 1.0 point (20% of 5)
    // 5 active = 0 points
    // Formula: max(0, 1.0 - (activeOrders / maxOrders))
    freeOrders := float64(maxOrders - activeOrders)
    return (freeOrders / float64(maxOrders)) * 1.0
}

Examples (max = 5 orders):
- 0 active → 1.0 point
- 1 active → 0.8 points
- 2 active → 0.6 points
- 3 active → 0.4 points
- 5 active → 0 points
```

### **Response Time Score (10%)**
```go
func responseScore(lastResponseSec int) float64 {
    // Faster = better
    // 0 sec = 0.5 point (10% of 5)
    // 5 sec = 0 points
    // Formula: max(0, 0.5 - (lastResponseSec / 10))
    return max(0, 0.5 - float64(lastResponseSec / 10))
}

Examples:
- 1 sec → 0.45 points
- 2 sec → 0.4 points
- 5 sec → 0 points
```

---

## 💻 CODE IMPLEMENTATION

### **Driver Selection Function**

```go
type DriverScore struct {
    DriverID     int64
    Name         string
    Rating       float64
    Distance     float64
    Availability float64
    Response     float64
    TotalScore   float64
}

func (b *Bot) selectBestDriver(orderID int64, order *models.Order) (*int64, error) {
    // 1. Get all drivers
    allDrivers, _ := b.Stg.User().GetAll(context.Background())
    
    // 2. Filter matching
    matchedDrivers := b.filterMatchingDrivers(allDrivers, order)
    if len(matchedDrivers) == 0 {
        return nil, fmt.Errorf("No drivers available")
    }
    
    // 3. Score each driver
    scores := make([]DriverScore, 0)
    for _, driver := range matchedDrivers {
        score := b.calculateDriverScore(driver, order)
        scores = append(scores, score)
    }
    
    // 4. Sort by total score (descending)
    sort.Slice(scores, func(i, j int) bool {
        return scores[i].TotalScore > scores[j].TotalScore
    })
    
    // 5. Return best driver ID
    bestDriver := scores[0]
    b.Log.Info("Best driver selected",
        logger.Int64("driver_id", bestDriver.DriverID),
        logger.String("name", bestDriver.Name),
        logger.Float64("score", bestDriver.TotalScore),
    )
    
    return &bestDriver.DriverID, nil
}

func (b *Bot) calculateDriverScore(driver *models.User, order *models.Order) DriverScore {
    score := DriverScore{
        DriverID: driver.ID,
        Name:     driver.FullName,
    }
    
    // Rating (40%)
    score.Rating = driver.Rating * (2.0 / 5.0)
    
    // Distance (30%)
    distance := b.calculateDistance(driver.Location, order.FromLocation)
    score.Distance = max(0, 1.5 - (distance / 3.33))
    
    // Availability (20%)
    activeOrders := b.Stg.Order().CountActiveByDriver(driver.ID)
    score.Availability = (float64(5-activeOrders) / 5.0) * 1.0
    
    // Response Time (10%)
    lastResponse := int(time.Since(driver.LastResponseTime).Seconds())
    score.Response = max(0, 0.5 - float64(lastResponse / 10))
    
    // Total
    score.TotalScore = score.Rating + score.Distance + score.Availability + score.Response
    
    return score
}
```

---

## 🎯 FINAL PROCESS

```
ADMIN CLICKS: "✅ ОДОБРИТЬ ЗАКАЗ"
    ↓
Order status: "pending" → "active"
    ↓
System calls: selectBestDriver(order)
    ├─ Filter 100 drivers → 40 matched
    ├─ Score all 40
    ├─ Sort by score
    └─ Return top 1 driver (Shukhrat)
    ↓
Send notification to TOP 3 DRIVERS:
    1. Shukhrat (3.18) [First choice]
    2. Alisher (3.02)
    3. Dilshod (2.98)
    ↓
FIRST DRIVER TO CLICK ACCEPTS:
    If Shukhrat clicks → "✅ Order accepted!"
    If Alisher clicks → "✅ Order accepted!"
    If Dilshod clicks → "✅ Order accepted!"
    If none click in 10 sec → Try next 3 drivers
    ↓
ORDER COMPLETED ✅
```

---

## 📋 ADVANTAGES OF THIS SYSTEM

✅ **Fair** - Best drivers get priority
✅ **Fast** - Top drivers notified immediately  
✅ **Responsive** - Fallback if top drivers busy
✅ **Data-driven** - Uses rating, location, availability
✅ **Scalable** - Works with 100+ drivers
✅ **No admin decision** - Automatic matching
✅ **No bottleneck** - Parallel notifications

---

## 🔧 IMPLEMENTATION CHECKLIST

To implement this:

```
[ ] Add location columns to users table (lat, lon)
[ ] Add rating column to users table
[ ] Create distance calculation function
[ ] Create scoring algorithm
[ ] Create selectBestDriver() function
[ ] Update handleApproveOrder() to call selectBestDriver()
[ ] Send notification to top 3 drivers instead of all 100
[ ] Add logging for score calculation
[ ] Test with sample data
[ ] Add fallback logic if drivers don't respond
```

---

## 📊 COMPARISON: Manual vs Automatic

| Aspect | Manual (Now) | Automatic (Proposed) |
|--------|-------------|-------------------|
| **Admin decision** | Yes, for each driver | No, automatic |
| **Time to assign** | 30+ sec (admin delay) | <1 sec |
| **Quality** | Depends on admin | Consistent scoring |
| **Fairness** | Not always fair | Very fair |
| **Scalability** | Doesn't scale (100 drivers) | Scales well |
| **Best driver match** | Maybe | Always |

---

## 🎓 SUMMARY

**100 ta driver'dan 1 ni tanlash uchun:**

1. ✅ Filter: Only matching drivers (status, tariff, route)
2. ✅ Score: Rate each driver (rating, distance, availability)
3. ✅ Sort: Find best score
4. ✅ Notify: Top 3 drivers (first to click, wins)
5. ✅ Fallback: If no response, try next tier

**Result**: Best driver gets order, automatically, without admin!

