package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"taxibot/config"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"taxibot/storage"

	"github.com/jackc/pgx/v5/pgxpool"
	tele "gopkg.in/telebot.v3"
)

type BotType string

const (
	BotTypeClient BotType = "client"
	BotTypeDriver BotType = "driver"
	BotTypeAdmin  BotType = "admin"
)

type UserSession struct {
	DBID           int64
	State          string
	OrderData      *models.Order
	TempString     string
	LastActionTime time.Time
}

type Bot struct {
	Type     BotType
	Bot      *tele.Bot
	DB       *pgxpool.Pool
	Log      logger.ILogger
	Cfg      *config.Config
	Stg      storage.IStorage
	Sessions map[int64]*UserSession
	Peers    map[BotType]*Bot // Map of other bots to communicate with
}

const (
	StateIdle       = "idle"
	StateFrom       = "awaiting_from"
	StateTo         = "awaiting_to"
	StateTariff     = "awaiting_tariff"
	StatePassengers = "awaiting_passengers"
	StateDateTime   = "awaiting_datetime"
	StateConfirm    = "awaiting_confirm"

	StateTariffAdd   = "awaiting_tariff_name"
	StateLocationAdd = "awaiting_location_name"

	StateDriverRouteFrom = "awaiting_driver_route_from"
	StateDriverRouteTo   = "awaiting_driver_route_to"

	StateAdminLogin    = "awaiting_admin_login"
	StateAdminPassword = "awaiting_admin_password"
)

func (b *Bot) handleWebApp(c tele.Context) error {
	data := c.Message().WebAppData.Data
	// data is a JSON string like {"action":"take_order","order_id":123}
	// Simplified parsing for now:
	if strings.Contains(data, "take_order") {
		parts := strings.Split(data, ":")
		if len(parts) > 2 {
			idStr := strings.Trim(parts[2], "}\" ")
			id, _ := strconv.ParseInt(idStr, 10, 64)
			// Trigger take logic same as callback
			return b.handleTakeOrderWithID(c, id)
		}
	}
	return nil
}

func (b *Bot) handleTakeOrderWithID(c tele.Context, id int64) error {
	session := b.Sessions[c.Sender().ID]
	var dbID int64
	if session != nil {
		dbID = session.DBID
	} else {
		user := b.getCurrentUser(c)
		dbID = user.ID
	}

	// 1. Check if order is still active (not taken by someone else and approved)
	order, _ := b.Stg.Order().GetByID(context.Background(), id)
	if order == nil || order.Status != "active" {
		return c.Send("❌ Kechirasiz, ushbu buyurtma allaqachon olingan yoki bekor qilingan.")
	}

	// 2. Set status to 'wait_confirm' (Waiting for Admin approval of the match)
	// We'll update the order status and assign the driver_id temporarily (or use a separate request table, but let's use order table for simplicity)
	// We might need to store the candidate driver ID. Since we don't have a separate field, we can use `driver_id` but keep status non-final.
	// Let's rely on `driver_id` being set but status being `wait_confirm`.
	err := b.Stg.Order().TakeOrder(context.Background(), id, dbID) // Updates driver_id, status -> 'taken'. We need to override this behavior or update status manually after.
	// Actually TakeOrder sets to taken. Let's start with that then update to wait_confirm immediately.
	// Or better, do a custom query.
	if err != nil {
		return c.Send("❌ Xatolik: " + err.Error())
	}
	// Revert status to 'wait_confirm'
	b.DB.Exec(context.Background(), "UPDATE orders SET status='wait_confirm' WHERE id=$1", id)

	c.Send("⏳ So'rovingiz adminga yuborildi. Admin tasdiqlashini kuting...")

	// 3. Notify Admin
	driver, _ := b.Stg.User().Get(context.Background(), dbID)

	msg := fmt.Sprintf("🔔 <b>HAYDOVCHI BUYURTMANI OLMOQCHI</b>\n\n🆔 Buyurtma: #%d\n🚖 Haydovchi: <a href=\"tg://user?id=%d\">%s</a>\n📞 Tel: %s",
		id, driver.TelegramID, driver.FullName, *driver.Phone)

	b.notifyAdmin(id, msg, "match") // "match" type allows us to send specific buttons

	// 4. Notify Client (to keep them waiting)
	b.notifyUser(order.ClientID, "🚕 Haydovchi topildi! Operator tekshirib tasdiqlashini kuting...")

	return nil
}

func New(botType BotType, cfg *config.Config, stg storage.IStorage, log logger.ILogger) (*Bot, error) {
	token := cfg.TelegramBotToken
	if botType == BotTypeDriver {
		token = cfg.DriverBotToken
	} else if botType == BotTypeAdmin {
		token = cfg.AdminBotToken
	}

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}
	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}
	bot := &Bot{
		Type:     botType,
		Bot:      b,
		DB:       stg.GetPool(),
		Log:      log,
		Cfg:      cfg,
		Stg:      stg,
		Sessions: make(map[int64]*UserSession),
		Peers:    make(map[BotType]*Bot),
	}
	bot.registerHandlers()
	return bot, nil
}

func (b *Bot) Start() {
	b.Log.Info(fmt.Sprintf("🤖 %s Bot Started...", b.Type))
	b.Bot.Start()
}

var messages = map[string]map[string]string{
	"uz": {
		"welcome":       "👋 Assalomu alaykum! Tizimga xush kelibsiz.",
		"contact_msg":   "Ro'yxatdan o'tish uchun telefon raqamingizni yuboring:",
		"share_contact": "📱 Raqamni Ulashish",
		"registered":    "🎉 Ro'yxatdan muvaffaqiyatli o'tdingiz!",
		"blocked":       "🚫 Sizning hisobingiz bloklangan.",
		"no_entry":      "🚫 Ushbu bot faqat haydovchilar va adminlar uchun.",
		"menu_client":   "👤 Mijoz menyusi:",
		"menu_driver":   "🚖 Haydovchi menyusi:",
		"menu_admin":    "🛠 Admin paneli:",
		"order_from":    "📍 Qayerdan olasiz? (Shahar/tuman nomi)",
		"order_to":      "🏁 Qayerga borasiz? (Shahar/tuman nomi)",
		"order_tariff":  "🚕 Tarifni tanlang:",
		"order_pass":    "👥 Yo'lovchilar sonini kiriting:",
		"order_time":    "📅 Ketish vaqti va sanasini kiriting (Masalan: Bugun 18:00):",
		"order_confirm": "💰 Buyurtma tafsilotlari:\nNarx: %d %s\n\nTasdiqlaysizmi?",
		"order_created": "✅ Buyurtmangiz qabul qilindi!",
		"no_orders":     "📭 Hozircha faol buyurtmalar yo'q.",
		"notif_new":     "🔔 YANGI ZAKAZ!\n🆔 #%d\n💰 Narx: %d %s\n📍 Yo'l: %s",
		"notif_taken":   "🚖 Buyurtmangiz haydovchi tomonidan qabul qilindi!\n\n🆔 ID: #%d\n🚗 Haydovchi: %s\n📞 Tel: %s\n👤 Profil: %s",
		"notif_done":    "🏁 Buyurtmangiz muvaffaqiyatli yakunlandi. Rahmat!",
		"notif_cancel":  "⚠️ Buyurtma #%d bekor qilindi.",
	},
}

func (b *Bot) registerHandlers() {
	b.Bot.Handle("/start", b.handleStart)

	// Client Handlers
	if b.Type == BotTypeClient {
		b.Bot.Handle(tele.OnContact, b.handleContact)
		b.Bot.Handle("➕ Zakaz berish", b.handleOrderStart)
		b.Bot.Handle("📋 Mening zakazlarim", b.handleMyOrders)
	}

	// Driver Handlers
	if b.Type == BotTypeDriver {
		b.Bot.Handle(tele.OnContact, b.handleContact)
		b.Bot.Handle("📦 Faol zakazlar", b.handleActiveOrders)
		b.Bot.Handle("📋 Mening zakazlarim", b.handleMyOrdersDriver)
		b.Bot.Handle("📍 Yo'nalishlarim", b.handleDriverRoutes)
		b.Bot.Handle("🚕 Tariflarim", b.handleDriverTariffs)
		b.Bot.Handle("🔍 Sanadan qidirish", b.handleDriverCalendarSearch)
		b.Bot.Handle("🏠 Asosiy menyu", b.handleStart)
	}

	// Admin Handlers
	if b.Type == BotTypeAdmin {
		b.Bot.Handle(tele.OnContact, b.handleContact)
		b.Bot.Handle("👥 Userlar", b.handleAdminUsers)
		b.Bot.Handle("📦 Jami zakazlar", b.handleAdminOrders)
		b.Bot.Handle("⚙️ Tariflar", b.handleAdminTariffs)
		b.Bot.Handle("🗺 Shaharlar", b.handleAdminLocations)
		b.Bot.Handle("📊 Statistika", b.handleAdminStats)
		b.Bot.Handle("➕ Tarif qo'shish", b.handleTariffAddStart)
		b.Bot.Handle("➕ Shahar qo'shish", b.handleLocationAddStart)
		b.Bot.Handle("🏠 Asosiy menyu", b.handleStart)
	}

	b.Bot.Handle(tele.OnCallback, b.handleCallback)
	b.Bot.Handle(tele.OnText, b.handleText)
	b.Bot.Handle(tele.OnWebApp, b.handleWebApp)
}

func (b *Bot) handleStart(c tele.Context) error {
	b.Log.Info(fmt.Sprintf("Start command received from %d (%s)", c.Sender().ID, c.Sender().Username))
	ctx := context.Background()
	user, _ := b.Stg.User().GetOrCreate(ctx, c.Sender().ID, c.Sender().Username, fmt.Sprintf("%s %s", c.Sender().FirstName, c.Sender().LastName))

	isAdmin := (b.Cfg.AdminID != 0 && c.Sender().ID == b.Cfg.AdminID) ||
		(b.Cfg.AdminUsername != "" && c.Sender().Username == b.Cfg.AdminUsername)

	if isAdmin && user.Role != "admin" {
		b.Stg.User().UpdateRole(ctx, c.Sender().ID, "admin")
		user, _ = b.Stg.User().Get(ctx, c.Sender().ID)
	}

	if (b.Type == BotTypeDriver || b.Type == BotTypeAdmin) && !isAdmin && user.Role == "client" && user.Status != "pending" {
		return c.Send("🚫 <b>Kirish taqiqlandi!</b>\n\nSiz mijoz sifatida ro'yxatdan o'tgansiz.\n\n👇 Iltimos, mijozlar botiga o'ting:\n@clienttaxigo_bot", tele.ModeHTML)
	}

	if b.Type == BotTypeClient && user.Role == "driver" {
		return c.Send("🚫 <b>Kirish taqiqlandi!</b>\n\nSiz haydovchi sifatida ro'yxatdan o'tgansiz.\n\n👇 Iltimos, haydovchilar botiga o'ting:\n@drivertaxisgo_bot", tele.ModeHTML)
	}

	// Always initialize/reset session on /start
	b.Sessions[c.Sender().ID] = &UserSession{
		DBID:      user.ID,
		State:     StateIdle,
		OrderData: &models.Order{ClientID: user.ID},
	}

	if user.Status == "pending" {
		menu := &tele.ReplyMarkup{ResizeKeyboard: true}
		menu.Reply(menu.Row(menu.Contact(messages["uz"]["share_contact"])))
		return c.Send(messages["uz"]["contact_msg"], menu)
	}

	return b.showMenu(c, user)
}

func (b *Bot) handleContact(c tele.Context) error {
	b.Log.Info(fmt.Sprintf("Contact received from %d", c.Sender().ID))
	if c.Message().Contact.UserID != c.Sender().ID {
		return c.Send("O'zingizni raqamingizni yuboring.")
	}
	ctx := context.Background()
	b.Stg.User().UpdatePhone(ctx, c.Sender().ID, c.Message().Contact.PhoneNumber)

	// If registering via Driver Bot, set role to driver
	if b.Type == BotTypeDriver {
		b.Stg.User().UpdateRole(ctx, c.Sender().ID, "driver")
	}

	b.Stg.User().UpdateStatus(ctx, c.Sender().ID, "active")
	user, _ := b.Stg.User().Get(ctx, c.Sender().ID)

	c.Send(messages["uz"]["registered"], tele.RemoveKeyboard)

	// If it's a driver bot and user is a driver (or just became one), start route setup
	if b.Type == BotTypeDriver && user.Role == "driver" {
		session := b.Sessions[c.Sender().ID]
		if session == nil {
			b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
			session = b.Sessions[c.Sender().ID]
		}
		if err := b.showMenu(c, user); err != nil {
			b.Log.Error("Failed to show menu", logger.Error(err))
		}
		return b.handleAddRouteStart(c, session)
	}

	return b.showMenu(c, user)
}

func (b *Bot) showMenu(c tele.Context, user *models.User) error {
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}

	if b.Type == BotTypeClient {
		menu.Reply(
			menu.Row(menu.Text("➕ Zakaz berish")),
			menu.Row(menu.Text("📋 Mening zakazlarim")),
		)
		return c.Send(messages["uz"]["menu_client"], &tele.SendOptions{ReplyMarkup: menu})
	}

	if user.Role == "admin" {
		menu.Reply(
			menu.Row(menu.Text("👥 Userlar"), menu.Text("📦 Jami zakazlar")),
			menu.Row(menu.Text("⚙️ Tariflar"), menu.Text("🗺 Shaharlar")),
			menu.Row(menu.Text("📊 Statistika")),
			menu.Row(menu.Text("📦 Faol zakazlar"), menu.Text("📋 Mening zakazlarim")),
			menu.Row(menu.Text("🏠 Asosiy menyu")),
		)
		return c.Send(messages["uz"]["menu_admin"], &tele.SendOptions{ReplyMarkup: menu})
	}

	// Driver Menu
	menu.Reply(
		menu.Row(menu.Text("📦 Faol zakazlar")),
		menu.Row(menu.Text("📍 Yo'nalishlarim"), menu.Text("🚕 Tariflarim")),
		menu.Row(menu.Text("� Sanadan qidirish")),
		menu.Row(menu.Text("�📋 Mening zakazlarim")),
	)
	return c.Send(messages["uz"]["menu_driver"], &tele.SendOptions{ReplyMarkup: menu})
}

func (b *Bot) handleOrderStart(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]

		// Set initial time to avoid immediate block if just created (though LastActionTime is zero value, Since() > 1.5s)
	}

	// Debounce: Preventive double-click protection (1.5 seconds)
	if time.Since(session.LastActionTime) < 1500*time.Millisecond {
		return nil
	}
	session.LastActionTime = time.Now()

	session.State = StateFrom
	session.OrderData = &models.Order{ClientID: session.DBID}

	locations, _ := b.Stg.Location().GetAll(context.Background())
	menu := &tele.ReplyMarkup{}

	var rows []tele.Row
	var currentRow []tele.Btn
	for i, l := range locations {
		currentRow = append(currentRow, menu.Data(l.Name, fmt.Sprintf("cl_f_%d", l.ID)))
		if (i+1)%3 == 0 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}

	menu.Inline(rows...)
	return c.Send(messages["uz"]["order_from"], menu, tele.ModeHTML)
}

func (b *Bot) handleActiveOrders(c tele.Context) error {
	orders, _ := b.Stg.Order().GetActiveOrders(context.Background())
	if len(orders) == 0 {
		return c.Send(messages["uz"]["no_orders"])
	}

	for _, o := range orders {
		timeStr := "Noma'lum"
		if o.PickupTime != nil {
			timeStr = o.PickupTime.Format("02.01.2006 15:04")
		}

		txt := fmt.Sprintf("📦 <b>YANGI ZAKAZ #%d</b>\n\n📍 Yo'nalish: <b>%s ➡️ %s</b>\n💰 Narx: <b>%d %s</b>\n👥 Yo'lovchilar: <b>%d</b>\n🕒 Vaqt: <b>%s</b>",
			o.ID, o.FromLocationName, o.ToLocationName, o.Price, o.Currency, o.Passengers, timeStr)

		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(menu.Data("📥 Zakazni olish", fmt.Sprintf("take_%d", o.ID))))
		c.Send(txt, menu, tele.ModeHTML)
	}
	return nil
}

func (b *Bot) handleMyOrdersDriver(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}

	orders, _ := b.Stg.Order().GetDriverOrders(context.Background(), session.DBID)
	if len(orders) == 0 {
		return c.Send("Sizda olingan zakazlar yo'q.")
	}

	for _, o := range orders {
		timeStr := "Noma'lum"
		if o.PickupTime != nil {
			timeStr = o.PickupTime.Format("02.01.2006 15:04")
		}

		txt := fmt.Sprintf("🚖 <b>ZAKAZ #%d</b>\n📍 %s ➡️ %s\n👥 Yo'lovchilar: %d\n💰 Narx: %d %s\n📅 Vaqt: %s\n📊 Status: %s",
			o.ID, o.FromLocationName, o.ToLocationName, o.Passengers, o.Price, o.Currency, timeStr, o.Status)

		menu := &tele.ReplyMarkup{}
		if o.Status == "taken" {
			menu.Inline(menu.Row(menu.Data("✅ Yakunlash", fmt.Sprintf("complete_%d", o.ID))))
		}
		c.Send(txt, menu, tele.ModeHTML)
	}
	return nil
}

func (b *Bot) handleAdminUsers(c tele.Context) error {
	return b.showUsersPage(c, 0)
}

func (b *Bot) showUsersPage(c tele.Context, page int) error {
	const limit = 5
	users, _ := b.Stg.User().GetAll(context.Background())
	totalPages := (len(users) + limit - 1) / limit

	if page < 0 {
		page = 0
	}
	if page >= totalPages && totalPages > 0 {
		page = totalPages - 1
	}

	start := page * limit
	end := start + limit
	if end > len(users) {
		end = len(users)
	}

	if len(users) == 0 {
		return c.Send("👥 Userlar topilmadi.")
	}

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("👥 <b>Userlar Ro'yxati (%d/%d):</b>\n\n", page+1, totalPages))

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	for i := start; i < end; i++ {
		u := users[i]
		phone := "Noma'lum"
		if u.Phone != nil {
			phone = *u.Phone
		}

		msg.WriteString(fmt.Sprintf("🆔 <b>%d</b> | %s\n📞 %s | Role: <b>%s</b> | Status: <b>%s</b>\n", u.TelegramID, u.FullName, phone, u.Role, u.Status))
		msg.WriteString("------------------------------\n")

		btnRole := menu.Data(fmt.Sprintf("� Role %d", u.ID), fmt.Sprintf("adm_role_%d_%d", u.TelegramID, page))
		btnStatus := menu.Data(fmt.Sprintf("🚫/✅ %d", u.ID), fmt.Sprintf("adm_stat_%d_%d", u.TelegramID, page))
		rows = append(rows, menu.Row(btnRole, btnStatus))
	}

	// Navigation
	var navRow []tele.Btn
	if page > 0 {
		navRow = append(navRow, menu.Data("⬅️ Oldingi", fmt.Sprintf("users_page_%d", page-1)))
	}
	if page < totalPages-1 {
		navRow = append(navRow, menu.Data("Keyingi ➡️", fmt.Sprintf("users_page_%d", page+1)))
	}
	// Always add Back button
	navRow = append(navRow, menu.Data("⬅️ Orqaga", "admin_back"))

	if len(navRow) > 0 {
		rows = append(rows, menu.Row(navRow...))
	}

	menu.Inline(rows...)

	if c.Callback() != nil {
		return c.Edit(msg.String(), menu, tele.ModeHTML)
	}
	return c.Send(msg.String(), menu, tele.ModeHTML)
}

func (b *Bot) handleAdminOrders(c tele.Context) error {
	return b.showOrdersPage(c, 0)
}

func (b *Bot) showOrdersPage(c tele.Context, page int) error {
	const limit = 5
	orders, _ := b.Stg.Order().GetAll(context.Background())
	// In real app, use DB offset/limit. Here purely slicing.
	// Sort by ID desc (newest first)
	for i := 0; i < len(orders)/2; i++ {
		j := len(orders) - i - 1
		orders[i], orders[j] = orders[j], orders[i]
	}

	totalPages := (len(orders) + limit - 1) / limit
	if page < 0 {
		page = 0
	}
	if page >= totalPages && totalPages > 0 {
		page = totalPages - 1
	}

	start := page * limit
	end := start + limit
	if end > len(orders) {
		end = len(orders)
	}

	if len(orders) == 0 {
		return c.Send("📦 Zakazlar topilmadi.")
	}

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("📦 <b>Barcha Zakazlar (%d/%d):</b>\n\n", page+1, totalPages))

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	for i := start; i < end; i++ {
		o := orders[i]
		msg.WriteString(fmt.Sprintf("🔹 <b>#%d</b> | %s\n📍 %s -> %s\n💰 %d %s\n\n", o.ID, o.Status, o.FromLocationName, o.ToLocationName, o.Price, o.Currency))

		if o.Status != "completed" && o.Status != "cancelled" {
			rows = append(rows, menu.Row(menu.Data(fmt.Sprintf("❌ Bekor qilish #%d", o.ID), fmt.Sprintf("adm_cancel_%d_%d", o.ID, page))))
		}
	}

	var navRow []tele.Btn
	if page > 0 {
		navRow = append(navRow, menu.Data("⬅️ Oldingi", fmt.Sprintf("orders_page_%d", page-1)))
	}
	if page < totalPages-1 {
		navRow = append(navRow, menu.Data("Keyingi ➡️", fmt.Sprintf("orders_page_%d", page+1)))
	}
	// Always add Back button
	navRow = append(navRow, menu.Data("⬅️ Orqaga", "admin_back"))

	if len(navRow) > 0 {
		rows = append(rows, menu.Row(navRow...))
	}

	menu.Inline(rows...)
	if c.Callback() != nil {
		return c.Edit(msg.String(), menu, tele.ModeHTML)
	}
	return c.Send(msg.String(), menu, tele.ModeHTML)
}

func (b *Bot) handleAdminTariffs(c tele.Context) error {
	tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(menu.Row(menu.Text("➕ Tarif qo'shish")), menu.Row(menu.Text("🏠 Asosiy menyu")))

	var msg strings.Builder
	msg.WriteString("⚙️ <b>Mavjud Tariflar:</b>\n\n")
	for i, t := range tariffs {
		msg.WriteString(fmt.Sprintf("%d. 🚕 <b>%s</b>\n", i+1, t.Name))
	}

	return c.Send(msg.String(), menu, tele.ModeHTML)
}

func (b *Bot) handleAdminLocations(c tele.Context) error {
	locations, _ := b.Stg.Location().GetAll(context.Background())
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(menu.Row(menu.Text("➕ Shahar qo'shish")), menu.Row(menu.Text("🏠 Asosiy menyu")))

	var msg strings.Builder
	msg.WriteString("🗺 <b>Mavjud Shaharlar:</b>\n\n")
	for i, l := range locations {
		msg.WriteString(fmt.Sprintf("%d. 📍 <b>%s</b>\n", i+1, l.Name))
	}

	return c.Send(msg.String(), menu, tele.ModeHTML)
}

func (b *Bot) handleAdminStats(c tele.Context) error {
	orders, _ := b.Stg.Order().GetAll(context.Background())
	users, _ := b.Stg.User().GetAll(context.Background())
	active := 0
	for _, o := range orders {
		if o.Status == "active" || o.Status == "taken" {
			active++
		}
	}
	return c.Send(fmt.Sprintf("📊 STATISTICS\n\nJami foydalanuvchilar: %d\nFaol zakazlar: %d\nJami zakazlar: %d", len(users), active, len(orders)))
}

func (b *Bot) handleText(c tele.Context) error {
	session, ok := b.Sessions[c.Sender().ID]
	if !ok || session.State == StateIdle {
		return nil
	}

	b.Log.Info("Handle Text", logger.String("text", c.Text()), logger.String("state", session.State))

	switch session.State {
	case StateFrom:
		session.TempString = c.Text()
		session.State = StateTo
		return c.Send(messages["uz"]["order_to"])
	case StateTo:
		session.TempString = session.TempString + " ➡️ " + c.Text()
		session.State = StateTariff
		tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		var currentRow []tele.Btn
		for i, t := range tariffs {
			currentRow = append(currentRow, menu.Data(t.Name, fmt.Sprintf("tf_%d", t.ID)))
			if (i+1)%2 == 0 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = []tele.Btn{}
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		menu.Inline(rows...)
		return c.Send(messages["uz"]["order_tariff"], menu)
	case StateTariffAdd:
		b.Stg.Tariff().Create(context.Background(), c.Text())
		session.State = StateIdle
		return b.showMenu(c, b.getCurrentUser(c))
	case StateLocationAdd:
		b.Stg.Location().Create(context.Background(), c.Text())
		session.State = StateIdle
		return b.showMenu(c, b.getCurrentUser(c))
	case StateAdminLogin:
		if c.Text() == "zarif" {
			session.State = StateAdminPassword
			return c.Send("🔑 Parolni kiriting:")
		} else {
			return c.Send("❌ Login xato! Qaytadan kiriting:")
		}
	case StateAdminPassword:
		if c.Text() == "1234" {
			// Success
			b.Stg.User().UpdateRole(context.Background(), session.DBID, "admin")
			session.State = StateIdle
			user, _ := b.Stg.User().Get(context.Background(), session.DBID)
			c.Send("✅ Muvaffaqiyatli kirdingiz!")
			return b.showMenu(c, user)
		} else {
			return c.Send("❌ Parol xato! Qaytadan kiriting:")
		}
	}
	return nil
}

func (b *Bot) handleCallback(c tele.Context) error {
	data := strings.TrimSpace(c.Callback().Data)

	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle, OrderData: &models.Order{ClientID: user.ID}}
		session = b.Sessions[c.Sender().ID]
	}
	if session.OrderData == nil {
		session.OrderData = &models.Order{ClientID: session.DBID}
	}

	b.Log.Info("Handle Callback",
		logger.String("data", data),
		logger.Int64("from_id", session.OrderData.FromLocationID),
		logger.Int64("to_id", session.OrderData.ToLocationID),
	)

	// Guard: Check for session loss during order flow
	// If the user clicks a button that requires previous state (like FromLocationID) but it's missing (due to restart),
	// we should stop them and ask to restart.
	// cl_f_ is the entry point, so we don't block it.
	isOrderFlowCallback := strings.HasPrefix(data, "cl_t_") ||
		strings.HasPrefix(data, "tf_") ||
		strings.HasPrefix(data, "cal_") ||
		strings.HasPrefix(data, "time_") ||
		strings.HasPrefix(data, "confirm_")

	if isOrderFlowCallback && session.OrderData.FromLocationID == 0 {

		c.Delete() // Delete the stale message/keyboard
		return c.Send("⚠️ <b>Sessiya yangilandi.</b>\n\nBot qayta ishga tushgani sababli, iltimos, buyurtmani boshqatdan shakllantiring:\n/start", tele.ModeHTML)
	}

	// Guard: Check for ToLocationID for steps that require it
	if (strings.HasPrefix(data, "tf_") ||
		strings.HasPrefix(data, "cal_") ||
		strings.HasPrefix(data, "time_") ||
		strings.HasPrefix(data, "confirm_")) && session.OrderData.ToLocationID == 0 {

		c.Delete()
		return c.Send("⚠️ <b>Sessiya yangilandi.</b>\n\nBot qayta ishga tushgani sababli, iltimos, buyurtmani boshqatdan shakllantiring:\n/start", tele.ModeHTML)
	}

	if strings.HasPrefix(data, "cl_f_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "cl_f_"), 10, 64)
		session.OrderData.FromLocationID = id
		session.State = StateTo

		locations, _ := b.Stg.Location().GetAll(context.Background())
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		var currentRow []tele.Btn
		count := 0
		for _, l := range locations {
			if l.ID != id {
				currentRow = append(currentRow, menu.Data(l.Name, fmt.Sprintf("cl_t_%d", l.ID)))
				count++
				if count%3 == 0 {
					rows = append(rows, menu.Row(currentRow...))
					currentRow = []tele.Btn{}
				}
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		menu.Inline(rows...)
		c.Respond(&tele.CallbackResponse{})
		return c.Edit(messages["uz"]["order_to"], menu, tele.ModeHTML)
	}

	if strings.HasPrefix(data, "cl_t_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "cl_t_"), 10, 64)
		session.OrderData.ToLocationID = id
		session.State = StateTariff

		tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		var currentRow []tele.Btn
		for i, t := range tariffs {
			currentRow = append(currentRow, menu.Data(t.Name, fmt.Sprintf("tf_%d", t.ID)))
			if (i+1)%2 == 0 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = []tele.Btn{}
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		menu.Inline(rows...)

		// Get location names for temp string display
		from, _ := b.Stg.Location().GetByID(context.Background(), session.OrderData.FromLocationID)
		to, _ := b.Stg.Location().GetByID(context.Background(), session.OrderData.ToLocationID)

		fromName := "Noma'lum"
		if from != nil {
			fromName = from.Name
		}
		toName := "Noma'lum"
		if to != nil {
			toName = to.Name
		}

		session.TempString = fmt.Sprintf("%s ➡️ %s", fromName, toName)
		return c.Edit(messages["uz"]["order_tariff"], menu)
	}

	if strings.HasPrefix(data, "tf_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "tf_"), 10, 64)
		session.OrderData.TariffID = id
		session.OrderData.Passengers = 1 // Default to 1 passenger
		session.State = StateDateTime

		// Show calendar for current month
		now := time.Now()
		return b.generateCalendar(c, now.Year(), int(now.Month()))
	}

	// Passenger selection removed - now defaults to 1

	if strings.HasPrefix(data, "nav_") {
		// Calendar navigation: nav_YYYY_M
		parts := strings.Split(data, "_")
		year, _ := strconv.Atoi(parts[1])
		month, _ := strconv.Atoi(parts[2])
		return b.generateCalendar(c, year, month)
	}

	if strings.HasPrefix(data, "cal_") {
		// Date selected: cal_2024-02-15
		dateStr := strings.TrimPrefix(data, "cal_")
		session.TempString = dateStr

		// Show time selection
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		var currentRow []tele.Btn

		// Use Moscow time (UTC+3)
		loc := time.FixedZone("Europe/Moscow", 3*60*60)
		now := time.Now().In(loc)

		currentHour := -1
		if dateStr == now.Format("2006-01-02") {
			currentHour = now.Hour()
		}

		for h := 0; h <= 23; h++ {
			timeStr := fmt.Sprintf("%02d:00", h)
			var btn tele.Btn
			if h <= currentHour {
				btn = menu.Data("🔒 "+timeStr, "ignore")
			} else {
				btn = menu.Data(timeStr, fmt.Sprintf("time_%s", timeStr))
			}
			currentRow = append(currentRow, btn)
			if len(currentRow) == 4 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = []tele.Btn{}
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		menu.Inline(rows...)
		c.Respond(&tele.CallbackResponse{})
		return c.Edit("� Soatni tanlang:", menu)
	}

	if strings.HasPrefix(data, "take_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "take_"), 10, 64)
		b.Bot.Edit(c.Callback().Message, "✅ Buyurtma qabul qilindi!")
		return b.handleTakeOrderWithID(c, id)
	}

	if strings.HasPrefix(data, "complete_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "complete_"), 10, 64)
		b.Stg.Order().CompleteOrder(context.Background(), id)
		b.Bot.Edit(c.Callback().Message, "🏁 Buyurtma yakunlandi!")
		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order != nil {
			b.notifyUser(order.ClientID, messages["uz"]["notif_done"])
		}
		return c.Respond()
	}

	if strings.HasPrefix(data, "cancel_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "cancel_"), 10, 64)
		b.Stg.Order().CancelOrder(context.Background(), id)
		b.Bot.Edit(c.Callback().Message, "❌ Bekor qilindi.")
		return c.Respond()
	}

	if data == "agenda_view" {
		return b.handleDriverAgenda(c)
	}
	if data == "native_cal" {
		now := time.Now()
		return b.generateCalendarWithPrefix(c, now.Year(), int(now.Month()), "sc_cal_")
	}

	if strings.HasPrefix(data, "sc_nav_") {
		parts := strings.Split(data, "_")
		if len(parts) >= 4 {
			year, _ := strconv.Atoi(parts[2])
			month, _ := strconv.Atoi(parts[3])
			return b.generateCalendarWithPrefix(c, year, month, "sc_cal_")
		}
		return nil
	}

	if strings.HasPrefix(data, "sc_cal_") {
		dateStr := strings.TrimPrefix(data, "sc_cal_")
		return b.handleDriverDateSearch(c, dateStr)
	}

	if strings.HasPrefix(data, "sc_cal_all_") {
		dateStr := strings.TrimPrefix(data, "sc_cal_all_")
		return b.handleDriverDateSearchAll(c, dateStr)
	}

	if strings.HasPrefix(data, "user_blk_") || strings.HasPrefix(data, "user_act_") || strings.HasPrefix(data, "set_role_") {
		return b.handleAdminCallbacks(c, data)
	}

	// Driver Callbacks
	if data == "add_route" {
		return b.handleAddRouteStart(c, session)
	}

	if data == "clear_routes" {
		b.Stg.Route().ClearRoutes(context.Background(), session.DBID)
		return b.handleDriverRoutes(c)
	}

	if strings.HasPrefix(data, "dr_f_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "dr_f_"), 10, 64)
		session.OrderData.FromLocationID = id // Use OrderData temporarily for route storage

		locations, _ := b.Stg.Location().GetAll(context.Background())
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		var currentRow []tele.Btn
		for i, l := range locations {
			if l.ID == id {
				continue
			}
			currentRow = append(currentRow, menu.Data(l.Name, fmt.Sprintf("dr_t_%d", l.ID)))
			if (i+1)%3 == 0 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = []tele.Btn{}
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		menu.Inline(rows...)
		c.Respond(&tele.CallbackResponse{})
		return c.Edit("<b>🏁 Qayerga borasiz?</b>\nShaharni tanlang:", menu, tele.ModeHTML)
	}

	if strings.HasPrefix(data, "dr_t_") {
		toID, _ := strconv.ParseInt(strings.TrimPrefix(data, "dr_t_"), 10, 64)
		fromID := session.OrderData.FromLocationID

		b.Stg.Route().AddRoute(context.Background(), session.DBID, fromID, toID)
		c.Respond(&tele.CallbackResponse{Text: "Yo'nalish qo'shildi!"})
		return b.handleDriverRoutes(c)
	}

	if strings.HasPrefix(data, "tgl_") {
		tariffID, _ := strconv.ParseInt(strings.TrimPrefix(data, "tgl_"), 10, 64)
		b.Stg.Tariff().Toggle(context.Background(), session.DBID, tariffID)

		// Refresh the tariff list
		tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
		enabled, _ := b.Stg.Tariff().GetEnabled(context.Background(), session.DBID)

		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		var currentRow []tele.Btn
		for i, t := range tariffs {
			icon := "🔴"
			if enabled[t.ID] {
				icon = "✅"
			}
			currentRow = append(currentRow, menu.Data(fmt.Sprintf("%s %s", icon, t.Name), fmt.Sprintf("tgl_%d", t.ID)))
			if (i+1)%2 == 0 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = []tele.Btn{}
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		menu.Inline(rows...)
		return c.Edit("<b>🚕 Tariflarim</b>\n\nQaysi tariflardan buyurtma olmoqchisiz? Tanlang:", menu, tele.ModeHTML)
	}

	switch data {
	case "confirm_yes":
		if session.OrderData == nil || session.OrderData.FromLocationID == 0 || session.OrderData.ToLocationID == 0 {
			b.Log.Warning("Invalid order data in session for confirm_yes", logger.Int64("user_id", c.Sender().ID))
			return c.Send("⚠️ <b>Xatolik:</b> Buyurtma ma'lumotlari topilmadi. Iltimos, /start bosib buyurtmani qaytadan shakllantiring.", tele.ModeHTML)
		}
		session.OrderData.Status = "active"
		order, err := b.Stg.Order().Create(context.Background(), session.OrderData)
		if err == nil {
			c.Send(messages["uz"]["order_created"])
			// New Flow: Notify Admin for approval
			adminMsg := fmt.Sprintf("🔔 <b>YANGI BUYURTMA (Tasdiqlash uchun)</b>\n\n🆔 #%d\n📍 %s ➡️ %s\n💰 %d %s\n👥 %d yo'lovchi\n📅 %s",
				order.ID, session.TempString, c.Text(), order.Price, order.Currency, order.Passengers, session.TempString)
			// Note: TempString might be overwritten or not perfect, ideally reconstruct from IDs.
			// Reconstructing strictly for Admin message:
			from, _ := b.Stg.Location().GetByID(context.Background(), session.OrderData.FromLocationID)
			to, _ := b.Stg.Location().GetByID(context.Background(), session.OrderData.ToLocationID)
			fromName, toName := "Noma'lum", "Noma'lum"
			if from != nil {
				fromName = from.Name
			}
			if to != nil {
				toName = to.Name
			}
			timeStr := "Hozir"
			if session.OrderData.PickupTime != nil {
				timeStr = session.OrderData.PickupTime.Format("02.01.2006 15:04")
			}

			adminMsg = fmt.Sprintf("🔔 <b>YANGI BUYURTMA (Tasdiqlash uchun)</b>\n\n🆔 #%d\n📍 %s ➡️ %s\n💰 Narx: %d %s\n👥 Yo'lovchilar: %d\n📅 Vaqt: %s",
				order.ID, fromName, toName, order.Price, order.Currency, order.Passengers, timeStr)

			b.notifyAdmin(order.ID, adminMsg)
			c.Send("⏳ Buyurtmangiz adminga yuborildi. Tasdiqlanishini kuting.")
		} else {
			b.Log.Error("Order creation failed", logger.Error(err))
			c.Send("❌ Buyurtma yaratishda xatolik yuz berdi.")
		}
		session.State = StateIdle
		return b.showMenu(c, b.getCurrentUser(c))
	case "confirm_no":
		session.State = StateIdle
		c.Send("❌ Bekor qilindi.")
		return b.showMenu(c, b.getCurrentUser(c))
	}

	if data == "ignore" {
		return c.Respond(&tele.CallbackResponse{Text: ""})
	}

	if strings.HasPrefix(data, "time_") {
		timeStr := strings.TrimPrefix(data, "time_") // "14:00"
		if session.TempString == "" {
			c.Delete()
			return c.Send("⚠️ <b>Xatolik:</b> Sana tanlanmagan. Iltimos, /start bosib buyurtmani qaytadan shakllantiring.", tele.ModeHTML)
		}

		fullTimeStr := fmt.Sprintf("%s %s", session.TempString, timeStr) // "2023-10-27 14:00"
		loc := time.FixedZone("Europe/Moscow", 3*60*60)
		parsedTime, err := time.ParseInLocation("2006-01-02 15:04", fullTimeStr, loc)
		if err != nil {
			b.Log.Error("Failed to parse time", logger.Error(err), logger.String("fullTimeStr", fullTimeStr))
			c.Delete()
			return c.Send("⚠️ <b>Xatolik:</b> Vaqt formati noto'g'ri. Iltimos, /start bosib buyurtmani qaytadan shakllantiring.", tele.ModeHTML)
		}

		// Convert to UTC for storage
		utcTime := parsedTime.UTC()

		session.OrderData.PickupTime = &utcTime
		session.OrderData.Price = 0 // Will be set by driver or standard? Let's keep 0 or default
		session.OrderData.Currency = "UZS"

		session.State = StateConfirm

		// Refresh names for confirmation message
		from, _ := b.Stg.Location().GetByID(context.Background(), session.OrderData.FromLocationID)
		to, _ := b.Stg.Location().GetByID(context.Background(), session.OrderData.ToLocationID)
		tariff, _ := b.Stg.Tariff().GetByID(context.Background(), session.OrderData.TariffID)

		fromName := "Noma'lum"
		if from != nil {
			fromName = from.Name
		}
		toName := "Noma'lum"
		if to != nil {
			toName = to.Name
		}
		tariffName := "Noma'lum"
		if tariff != nil {
			tariffName = tariff.Name
		}

		msg := fmt.Sprintf("<b>💰 Buyurtmani tasdiqlash</b>\n\n📍 <b>%s ➡️ %s</b>\n🚕 Tarif: <b>%s</b>\n👥 Yo'lovchilar: <b>%d</b>\n📅 Vaqt: <b>%s</b>\n\nTasdiqlaysizmi?",
			fromName, toName, tariffName, session.OrderData.Passengers, parsedTime.Format("02.01.2006 15:04"))

		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(
			menu.Data("✅ Tasdiqlash", "confirm_yes"),
			menu.Data("❌ Bekor qilish", "confirm_no"),
		))
		c.Respond(&tele.CallbackResponse{})
		return c.Edit(msg, menu, tele.ModeHTML)
	}

	return nil
}

func (b *Bot) handleAdminCallbacks(c tele.Context, data string) error {
	if b.Type != BotTypeAdmin {
		return nil
	}
	if strings.HasPrefix(data, "set_role_") {
		parts := strings.Split(data, "_")
		id, _ := strconv.ParseInt(parts[3], 10, 64)
		b.Stg.User().UpdateRole(context.Background(), id, parts[2])
		return c.Respond(&tele.CallbackResponse{Text: "OK"})
	}
	if strings.HasPrefix(data, "user_blk_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "user_blk_"), 10, 64)
		b.Stg.User().UpdateStatus(context.Background(), id, "blocked")
		return c.Respond(&tele.CallbackResponse{Text: "Bloklandi"})
	}
	if strings.HasPrefix(data, "user_act_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "user_act_"), 10, 64)
		b.Stg.User().UpdateStatus(context.Background(), id, "active")
		return c.Respond(&tele.CallbackResponse{Text: "Aktiv qilindi"})
	}

	// Pagination Handlers
	if strings.HasPrefix(data, "users_page_") {
		page, _ := strconv.Atoi(strings.TrimPrefix(data, "users_page_"))
		return b.showUsersPage(c, page)
	}
	if strings.HasPrefix(data, "orders_page_") {
		page, _ := strconv.Atoi(strings.TrimPrefix(data, "orders_page_"))
		return b.showOrdersPage(c, page)
	}

	// Admin Actions with Pagination Return
	if strings.HasPrefix(data, "adm_role_") {
		parts := strings.Split(strings.TrimPrefix(data, "adm_role_"), "_") // ID_PAGE
		teleID, _ := strconv.ParseInt(parts[0], 10, 64)
		page, _ := strconv.Atoi(parts[1])

		user, _ := b.Stg.User().Get(context.Background(), teleID)
		if user != nil {
			newRole := "driver"
			if user.Role == "driver" {
				newRole = "client"
			}
			b.Stg.User().UpdateRole(context.Background(), teleID, newRole)
		}
		return b.showUsersPage(c, page)
	}

	if strings.HasPrefix(data, "adm_stat_") {
		parts := strings.Split(strings.TrimPrefix(data, "adm_stat_"), "_") // ID_PAGE
		teleID, _ := strconv.ParseInt(parts[0], 10, 64)
		page, _ := strconv.Atoi(parts[1])

		user, _ := b.Stg.User().Get(context.Background(), teleID)
		if user != nil {
			newStatus := "blocked"
			if user.Status == "blocked" {
				newStatus = "active"
			}
			b.Stg.User().UpdateStatus(context.Background(), teleID, newStatus)
		}
		return b.showUsersPage(c, page)
	}

	if data == "admin_back" {
		return c.Delete()
	}

	if strings.HasPrefix(data, "adm_cancel_") {
		parts := strings.Split(strings.TrimPrefix(data, "adm_cancel_"), "_") // ID_PAGE
		orderID, _ := strconv.ParseInt(parts[0], 10, 64)
		page, _ := strconv.Atoi(parts[1])

		b.Stg.Order().CancelOrder(context.Background(), orderID)
		return b.showOrdersPage(c, page)
	}

	if strings.HasPrefix(data, "approve_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "approve_"), 10, 64)
		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order != nil {
			// Update status to active manually using DB execution for safety
			_, err := b.DB.Exec(context.Background(), "UPDATE orders SET status='active' WHERE id=$1", id)

			if err == nil {
				// Re-fetch to get updated status or just use data knowing it is active
				// Notify Drivers
				b.notifyDrivers(order.ID, order.FromLocationID, order.ToLocationID, order.TariffID,
					fmt.Sprintf(messages["uz"]["notif_new"], order.ID, order.Price, order.Currency, "")) // We don't have temp string here easily, maybe fetch route names or leave empty/generic

				// Notify Client
				b.notifyUser(order.ClientID, "✅ Buyurtmangiz admin tomonidan tasdiqlandi! Haydovchi qidirilmoqda...")
				return c.Edit("✅ Tasdiqlandi va haydovchilarga yuborildi.")
			}
		}
		return c.Edit("❌ Xatolik yuz berdi.")
	}

	if strings.HasPrefix(data, "reject_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "reject_"), 10, 64)
		b.Stg.Order().CancelOrder(context.Background(), id)
		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order != nil {
			b.notifyUser(order.ClientID, "❌ Buyurtmangiz admin tomonidan bekor qilindi.")
		}
		return c.Edit("❌ Bekor qilindi.")
	}

	// Match Approval (Driver <-> Client)
	if strings.HasPrefix(data, "approve_match_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "approve_match_"), 10, 64)

		// 1. Finalize Order
		b.DB.Exec(context.Background(), "UPDATE orders SET status='taken' WHERE id=$1", id)

		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order == nil {
			return c.Edit("❌ Buyurtma topilmadi.")
		}

		// 2. Notify Client (with Driver details)
		driver, _ := b.Stg.User().Get(context.Background(), *order.DriverID)
		if driver != nil {
			phone := "Noma'lum"
			if driver.Phone != nil {
				phone = *driver.Phone
			}
			profile := fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>", driver.TelegramID, driver.FullName)
			msg := fmt.Sprintf(messages["uz"]["notif_taken"], id, driver.FullName, phone, profile)
			b.notifyUser(order.ClientID, msg)
		}

		// 3. Notify Driver
		b.notifyDriverSpecific(*order.DriverID, fmt.Sprintf("✅ Admin buyurtmani tasdiqladi! (#%d)\nMijoz bilan bog'laning.", id))

		return c.Edit("✅ Muvaffaqiyatli biriktirildi.")
	}

	if strings.HasPrefix(data, "reject_match_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "reject_match_"), 10, 64)

		// 1. Reset Status to Active
		b.DB.Exec(context.Background(), "UPDATE orders SET status='active', driver_id=NULL WHERE id=$1", id)

		// 2. Notify Driver
		// We need to know who was the driver. We can find out from order before updating, or pass in callback.
		// Simply notifying generally or letting it be is okay, but ideally notify the rejected driver.
		// For simplicity, just reset.

		return c.Edit("❌ Rad etildi. Buyurtma qayta aktivlashtirildi.")
	}

	return nil
}

func (b *Bot) notifyDriverSpecific(driverID int64, text string) {
	target := b
	if b.Type != BotTypeDriver {
		if p, ok := b.Peers[BotTypeDriver]; ok {
			target = p
		} else {
			return
		}
	}
	var teleID int64
	b.DB.QueryRow(context.Background(), "SELECT telegram_id FROM users WHERE id=$1", driverID).Scan(&teleID)
	if teleID != 0 {
		target.Bot.Send(&tele.User{ID: teleID}, text, tele.ModeHTML)
	}
}

func (b *Bot) notifyUser(dbID int64, text string) {
	target := b
	if b.Type != BotTypeClient {
		if p, ok := b.Peers[BotTypeClient]; ok {
			target = p
		} else {
			b.Log.Error("Client bot peer not found for notification")
			return
		}
	}
	var teleID int64
	b.DB.QueryRow(context.Background(), "SELECT telegram_id FROM users WHERE id=$1", dbID).Scan(&teleID)
	if teleID != 0 {
		target.Bot.Send(&tele.User{ID: teleID}, text, tele.ModeHTML)
	}
}

func (b *Bot) notifyAdmin(orderID int64, text string, msgType ...string) {
	target := b
	if b.Type != BotTypeAdmin {
		if p, ok := b.Peers[BotTypeAdmin]; ok {
			target = p
		} else {
			b.Log.Error("Admin bot peer not found for notification")
			return
		}
	}

	// Create Approval Keyboard
	menu := &tele.ReplyMarkup{}

	if len(msgType) > 0 && msgType[0] == "match" {
		menu.Inline(menu.Row(
			menu.Data("✅ Tasdiqlash", fmt.Sprintf("approve_match_%d", orderID)),
			menu.Data("❌ Rad etish", fmt.Sprintf("reject_match_%d", orderID)),
		))
	} else {
		menu.Inline(menu.Row(
			menu.Data("✅ Tasdiqlash", fmt.Sprintf("approve_%d", orderID)),
			menu.Data("❌ Bekor qilish", fmt.Sprintf("reject_%d", orderID)),
		))
	}

	// Send to all admins
	admins, _ := b.Stg.User().GetAll(context.Background()) // Should filter for admins ideally
	for _, u := range admins {
		if u.Role == "admin" {
			target.Bot.Send(&tele.User{ID: u.TelegramID}, text, menu, tele.ModeHTML)
		}
	}
}

func (b *Bot) notifyDrivers(orderID, fromID, toID, tariffID int64, text string) {
	target := b
	if b.Type != BotTypeDriver {
		if p, ok := b.Peers[BotTypeDriver]; ok {
			target = p
		} else {
			b.Log.Error("Driver bot peer not found for notification")
			return
		}
	}

	// Get drivers explicitly matching the route
	routeDriversMap := make(map[int64]bool)
	routeDrivers, _ := b.Stg.Route().GetDriversByRoute(context.Background(), fromID, toID)
	for _, id := range routeDrivers {
		routeDriversMap[id] = true
	}

	targetIDs := make(map[int64]bool)
	users, _ := b.Stg.User().GetAll(context.Background())

	for _, u := range users {
		if u.Role != "driver" {
			continue
		}

		// Check tariff first
		enabled, _ := b.Stg.Tariff().GetEnabled(context.Background(), u.ID)
		if !enabled[tariffID] {
			continue
		}

		// Route Logic:
		// 1. If driver matches route -> Notify
		// 2. If driver has NO routes at all -> Notify (Default)
		// 3. If driver has routes but doesn't match -> Skip

		if routeDriversMap[u.ID] {
			targetIDs[u.ID] = true
			continue
		}

		// Check if driver has any routes
		driverRoutes, _ := b.Stg.Route().GetDriverRoutes(context.Background(), u.ID)
		if len(driverRoutes) == 0 {
			targetIDs[u.ID] = true
		}
	}

	menu := &tele.ReplyMarkup{}
	menu.Inline(menu.Row(
		menu.Data("📥 Zakazni olish", fmt.Sprintf("take_%d", orderID)),
		menu.Data("❌ Yopish", "close_msg"),
	))

	for id := range targetIDs {
		var teleID int64
		b.DB.QueryRow(context.Background(), "SELECT telegram_id FROM users WHERE id=$1", id).Scan(&teleID)
		if teleID != 0 {
			target.Bot.Send(&tele.User{ID: teleID}, text, menu, tele.ModeHTML)
		}
	}
}

func (b *Bot) handleMyOrders(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}

	orders, _ := b.Stg.Order().GetClientOrders(context.Background(), session.DBID)
	if len(orders) == 0 {
		return c.Send("Sizda zakazlar yo'q.")
	}
	for _, o := range orders {
		timeStr := "Noma'lum"
		if o.PickupTime != nil {
			timeStr = o.PickupTime.Format("02.01.2006 15:04")
		}

		txt := fmt.Sprintf("📦 <b>Zakaz #%d</b>\n📍 %s ➡️ %s\n👥 Yo'lovchilar: %d\n📅 Vaqt: %s\n📊 Status: %s",
			o.ID, o.FromLocationName, o.ToLocationName, o.Passengers, timeStr, o.Status)

		menu := &tele.ReplyMarkup{}
		if o.Status == "active" {
			menu.Inline(menu.Row(menu.Data("❌ Bekor qilish", fmt.Sprintf("cancel_%d", o.ID))))
		}
		c.Send(txt, menu, tele.ModeHTML)
	}
	return nil
}

func (b *Bot) handleTariffAddStart(c tele.Context) error {
	b.Sessions[c.Sender().ID].State = StateTariffAdd
	return c.Send("Tarif nomini yozing:")
}

func (b *Bot) handleLocationAddStart(c tele.Context) error {
	b.Sessions[c.Sender().ID].State = StateLocationAdd
	return c.Send("Shahar/Tuman nomini yozing:")
}

func (b *Bot) generateCalendar(c tele.Context, year, month int) error {
	return b.generateCalendarWithPrefix(c, year, month, "cal_")
}

func (b *Bot) generateCalendarWithPrefix(c tele.Context, year, month int, prefix string) error {
	// Month names in Uzbek
	monthNames := []string{"", "Yanvar", "Fevral", "Mart", "Aprel", "May", "Iyun",
		"Iyul", "Avgust", "Sentabr", "Oktabr", "Noyabr", "Dekabr"}

	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	// Header with month and year
	header := fmt.Sprintf("📅 %s %d", monthNames[month], year)

	// Week day names
	rows = append(rows, menu.Row(
		menu.Data("Du", "ignore"), menu.Data("Se", "ignore"), menu.Data("Ch", "ignore"),
		menu.Data("Pa", "ignore"), menu.Data("Ju", "ignore"), menu.Data("Sh", "ignore"), menu.Data("Ya", "ignore"),
	))

	// Get first day of week (0 = Sunday, 1 = Monday, etc.)
	firstWeekday := int(firstDay.Weekday())
	if firstWeekday == 0 {
		firstWeekday = 7 // Sunday becomes 7
	}
	firstWeekday-- // Adjust to Monday = 0

	var currentRow []tele.Btn

	// Add empty cells before first day
	for i := 0; i < firstWeekday; i++ {
		currentRow = append(currentRow, menu.Data(" ", "ignore"))
	}

	// Use fixed zone for Moscow (UTC+3)
	loc := time.FixedZone("Europe/Moscow", 3*60*60)
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Add all days of the month
	for day := 1; day <= lastDay.Day(); day++ {
		dayDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

		// Disable past dates
		// If dayDate is BEFORE today, it's past.
		// If dayDate EQUALS today, it's active.
		btnText := fmt.Sprintf("%d", day)
		var btn tele.Btn

		if dayDate.Before(today) {
			btn = menu.Data(btnText, "ignore")
		} else {
			btn = menu.Data(btnText, fmt.Sprintf("%s%s", prefix, dayDate.Format("2006-01-02")))
		}
		currentRow = append(currentRow, btn)

		if len(currentRow) == 7 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}

	// Fill remaining cells
	if len(currentRow) > 0 {
		for len(currentRow) < 7 {
			currentRow = append(currentRow, menu.Data(" ", "ignore"))
		}
		rows = append(rows, menu.Row(currentRow...))
	}

	// Navigation buttons
	prevMonth := month - 1
	prevYear := year
	if prevMonth < 1 {
		prevMonth = 12
		prevYear--
	}

	nextMonth := month + 1
	nextYear := year
	if nextMonth > 12 {
		nextMonth = 1
		nextYear++
	}

	// Only show navigation if not too far in past/future
	var navRow []tele.Btn
	navPrefix := "nav_"
	if prefix == "sc_cal_" {
		navPrefix = "sc_nav_"
	}

	if prevYear > time.Now().Year()-1 || (prevYear == time.Now().Year()-1 && prevMonth >= int(time.Now().Month())) {
		navRow = append(navRow, menu.Data(fmt.Sprintf("⬅️ %s", monthNames[prevMonth]), fmt.Sprintf("%s%d_%d", navPrefix, prevYear, prevMonth)))
	}
	if nextYear < time.Now().Year()+2 {
		navRow = append(navRow, menu.Data(fmt.Sprintf("%s ➡️", monthNames[nextMonth]), fmt.Sprintf("%s%d_%d", navPrefix, nextYear, nextMonth)))
	}

	if len(navRow) > 0 {
		rows = append(rows, menu.Row(navRow...))
	}

	menu.Inline(rows...)
	if c.Callback() != nil {
		c.Respond(&tele.CallbackResponse{})
		return c.Edit(header, menu)
	}
	return c.Send(header, menu)
}

func (b *Bot) getCurrentUser(c tele.Context) *models.User {
	u, err := b.Stg.User().Get(context.Background(), c.Sender().ID)
	if err == nil {
		return u
	}
	// If user not found, create (recover from DB wipe or new joiner clicking old button)
	sender := c.Sender()
	u, _ = b.Stg.User().GetOrCreate(context.Background(), sender.ID, sender.Username, fmt.Sprintf("%s %s", sender.FirstName, sender.LastName))
	return u
}
