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
	BotTypeClient      BotType = "client"
	BotTypeDriverAdmin BotType = "driver_admin"
)

type UserSession struct {
	DBID       int64
	State      string
	OrderData  *models.Order
	TempString string
}

type Bot struct {
	Type     BotType
	Bot      *tele.Bot
	DB       *pgxpool.Pool
	Log      logger.ILogger
	Cfg      *config.Config
	Stg      storage.IStorage
	Sessions map[int64]*UserSession
	Peer     *Bot // Link for cross-bot notifications
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

	err := b.Stg.Order().TakeOrder(context.Background(), id, dbID)
	if err != nil {
		return c.Send("❌ Buyurtmani qabul qilishda xatolik: " + err.Error())
	}

	c.Send("✅ Buyurtma qabul qilindi!")

	order, _ := b.Stg.Order().GetByID(context.Background(), id)
	driver, _ := b.Stg.User().Get(context.Background(), c.Sender().ID)

	// Sync with Google Calendar
	gCal := NewGoogleCalendarService()
	gCal.AddOrderToCalendar(order)

	if order != nil && driver != nil {
		phone := "Noma'lum"
		if driver.Phone != nil {
			phone = *driver.Phone
		}
		profile := fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>", driver.TelegramID, driver.FullName)
		if driver.Username != "" {
			profile += fmt.Sprintf(" (@%s)", driver.Username)
		}

		msg := fmt.Sprintf(messages["uz"]["notif_taken"], id, driver.FullName, phone, profile)
		b.notifyUser(order.ClientID, msg)
	}
	return nil
}

func New(botType BotType, cfg *config.Config, stg storage.IStorage, log logger.ILogger) (*Bot, error) {
	token := cfg.TelegramBotToken
	if botType == BotTypeDriverAdmin {
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

	if b.Type == BotTypeClient {
		b.Bot.Handle(tele.OnContact, b.handleContact)
		b.Bot.Handle("➕ Zakaz berish", b.handleOrderStart)
		b.Bot.Handle("📋 Mening zakazlarim", b.handleMyOrders)
	} else {
		b.Bot.Handle(tele.OnContact, b.handleContact)
		b.Bot.Handle("📦 Faol zakazlar", b.handleActiveOrders)
		b.Bot.Handle("📋 Mening zakazlarim", b.handleMyOrdersDriver)
		b.Bot.Handle("👥 Userlar", b.handleAdminUsers)
		b.Bot.Handle("📦 Jami zakazlar", b.handleAdminOrders)
		b.Bot.Handle("⚙️ Tariflar", b.handleAdminTariffs)
		b.Bot.Handle("🗺 Shaharlar", b.handleAdminLocations)
		b.Bot.Handle("📊 Statistika", b.handleAdminStats)
		b.Bot.Handle("➕ Tarif qo'shish", b.handleTariffAddStart)
		b.Bot.Handle("➕ Shahar qo'shish", b.handleLocationAddStart)
		b.Bot.Handle("📍 Yo'nalishlarim", b.handleDriverRoutes)
		b.Bot.Handle("🚕 Tariflarim", b.handleDriverTariffs)
		b.Bot.Handle("🔍 Sanadan qidirish", b.handleDriverCalendarSearch)
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

	if b.Type == BotTypeDriverAdmin && !isAdmin && user.Role == "client" && user.Status != "pending" {
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
	if c.Message().Contact.UserID != c.Sender().ID {
		return c.Send("O'zingizni raqamingizni yuboring.")
	}
	ctx := context.Background()
	b.Stg.User().UpdatePhone(ctx, c.Sender().ID, c.Message().Contact.PhoneNumber)

	// If registering via Driver Bot, set role to driver
	if b.Type == BotTypeDriverAdmin {
		b.Stg.User().UpdateRole(ctx, c.Sender().ID, "driver")
	}

	b.Stg.User().UpdateStatus(ctx, c.Sender().ID, "active")
	user, _ := b.Stg.User().Get(ctx, c.Sender().ID)

	c.Send(messages["uz"]["registered"], tele.RemoveKeyboard)

	// If it's a driver bot and user is a driver (or just became one), start route setup
	if b.Type == BotTypeDriverAdmin && user.Role == "driver" {
		session := b.Sessions[c.Sender().ID]
		if session == nil {
			b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
			session = b.Sessions[c.Sender().ID]
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
	}

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
	users, _ := b.Stg.User().GetAll(context.Background())
	for _, u := range users {
		phone := "Noma'lum"
		if u.Phone != nil {
			phone = *u.Phone
		}
		txt := fmt.Sprintf("👤 %s\n📞 %s\nRole: %s\nStatus: %s", u.FullName, phone, u.Role, u.Status)
		menu := &tele.ReplyMarkup{}
		btnRole := menu.Data("🚕 Driver qilish", fmt.Sprintf("set_role_driver_%d", u.TelegramID))
		if u.Role == "driver" {
			btnRole = menu.Data("👤 Client qilish", fmt.Sprintf("set_role_client_%d", u.TelegramID))
		}
		btnStatus := menu.Data("🚫 Block", fmt.Sprintf("user_blk_%d", u.TelegramID))
		if u.Status == "blocked" {
			btnStatus = menu.Data("✅ Activate", fmt.Sprintf("user_act_%d", u.TelegramID))
		}
		menu.Inline(menu.Row(btnRole, btnStatus))
		c.Send(txt, menu)
	}
	return nil
}

func (b *Bot) handleAdminOrders(c tele.Context) error {
	orders, _ := b.Stg.Order().GetAll(context.Background())
	for _, o := range orders {
		txt := fmt.Sprintf("📦 #%d | Status: %s\n💰 Narx: %d", o.ID, o.Status, o.Price)
		menu := &tele.ReplyMarkup{}
		if o.Status != "completed" && o.Status != "cancelled" {
			menu.Inline(menu.Row(menu.Data("❌ Bekor qilish (Admin)", fmt.Sprintf("cancel_%d", o.ID))))
			c.Send(txt, menu)
		} else {
			c.Send(txt)
		}
	}
	return nil
}

func (b *Bot) handleAdminTariffs(c tele.Context) error {
	tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(menu.Row(menu.Text("➕ Tarif qo'shish")), menu.Row(menu.Text("🏠 Asosiy menyu")))
	c.Send("⚙️ Tariflar boshqaruvi:", menu)
	for _, t := range tariffs {
		c.Send(fmt.Sprintf("🚕 %s", t.Name))
	}
	return nil
}

func (b *Bot) handleAdminLocations(c tele.Context) error {
	locations, _ := b.Stg.Location().GetAll(context.Background())
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(menu.Row(menu.Text("➕ Shahar qo'shish")), menu.Row(menu.Text("🏠 Asosiy menyu")))
	c.Send("🗺 Shaharlar boshqaruvi:", menu)
	for _, l := range locations {
		c.Send(fmt.Sprintf("📍 %s", l.Name))
	}
	return nil
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
		for _, t := range tariffs {
			rows = append(rows, menu.Row(menu.Data(t.Name, fmt.Sprintf("tf_%d", t.ID))))
		}
		menu.Inline(rows...)
		return c.Send(messages["uz"]["order_tariff"], menu)
	case StatePassengers:
		num, _ := strconv.Atoi(c.Text())
		session.OrderData.Passengers = num
		session.State = StateDateTime

		// Date selection menu
		menu := &tele.ReplyMarkup{}
		now := time.Now()

		today := now.Format("2006-01-02")
		tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
		afterTomorrow := now.AddDate(0, 0, 2).Format("2006-01-02")

		menu.Inline(
			menu.Row(menu.Data("Bugun", "date_"+today)),
			menu.Row(menu.Data("Ertaga", "date_"+tomorrow)),
			menu.Row(menu.Data("Indinga", "date_"+afterTomorrow)),
		)
		return c.Send("📅 Ketish kunini tanlang:", menu)
	case StateTariffAdd:
		b.Stg.Tariff().Create(context.Background(), c.Text())
		session.State = StateIdle
		return b.showMenu(c, b.getCurrentUser(c))
	case StateLocationAdd:
		b.Stg.Location().Create(context.Background(), c.Text())
		session.State = StateIdle
		return b.showMenu(c, b.getCurrentUser(c))
	}
	return nil
}

func (b *Bot) handleCallback(c tele.Context) error {
	data := strings.TrimSpace(c.Callback().Data)
	b.Log.Info(fmt.Sprintf("Handling Callback: %s from %d", data, c.Sender().ID))

	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle, OrderData: &models.Order{ClientID: user.ID}}
		session = b.Sessions[c.Sender().ID]
	}
	if session.OrderData == nil {
		session.OrderData = &models.Order{ClientID: session.DBID}
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
		return c.Edit(messages["uz"]["order_to"], menu, tele.ModeHTML)
	}

	if strings.HasPrefix(data, "cl_t_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "cl_t_"), 10, 64)
		session.OrderData.ToLocationID = id
		session.State = StateTariff

		tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		for _, t := range tariffs {
			rows = append(rows, menu.Row(menu.Data(t.Name, fmt.Sprintf("tf_%d", t.ID))))
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
		session.State = StatePassengers

		// Ask for passengers with buttons
		menu := &tele.ReplyMarkup{}
		menu.Inline(
			menu.Row(menu.Data("1", "pass_1"), menu.Data("2", "pass_2")),
			menu.Row(menu.Data("3", "pass_3"), menu.Data("4", "pass_4")),
		)
		b.Bot.Edit(c.Callback().Message, messages["uz"]["order_tariff"]) // Keep tariff text or update? Let's just update
		return c.Send(messages["uz"]["order_pass"], menu)
	}

	if strings.HasPrefix(data, "pass_") {
		num, _ := strconv.Atoi(strings.TrimPrefix(data, "pass_"))
		session.OrderData.Passengers = num
		session.State = StateDateTime

		// Show calendar for current month
		now := time.Now()
		return b.generateCalendar(c, now.Year(), int(now.Month()))
	}

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

		startTime := 5
		if dateStr == time.Now().Format("2006-01-02") {
			startTime = time.Now().Hour() + 1
		}

		for h := startTime; h <= 23; h++ {
			timeStr := fmt.Sprintf("%02d:00", h)
			currentRow = append(currentRow, menu.Data(timeStr, fmt.Sprintf("time_%s", timeStr)))
			if len(currentRow) == 4 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = []tele.Btn{}
			}
		}
		if len(currentRow) > 0 {
			rows = append(rows, menu.Row(currentRow...))
		}
		menu.Inline(rows...)
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
		for _, t := range tariffs {
			icon := "🔴"
			if enabled[t.ID] {
				icon = "✅"
			}
			rows = append(rows, menu.Row(menu.Data(fmt.Sprintf("%s %s", icon, t.Name), fmt.Sprintf("tgl_%d", t.ID))))
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
			b.notifyDrivers(order.ID, session.OrderData.FromLocationID, session.OrderData.ToLocationID, session.OrderData.TariffID, fmt.Sprintf(messages["uz"]["notif_new"], order.ID, order.Price, order.Currency, session.TempString))
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
		timeStr := strings.TrimPrefix(data, "time_")                     // "14:00"
		fullTimeStr := fmt.Sprintf("%s %s", session.TempString, timeStr) // "2023-10-27 14:00"
		parsedTime, _ := time.Parse("2006-01-02 15:04", fullTimeStr)

		session.OrderData.PickupTime = &parsedTime
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
		menu.Inline(menu.Row(menu.Data("✅ Tasdiqlash", "confirm_yes"), menu.Data("❌ Bekor qilish", "confirm_no")))
		return c.Edit(msg, menu, tele.ModeHTML)
	}

	return nil
}

func (b *Bot) handleAdminCallbacks(c tele.Context, data string) error {
	if b.Type != BotTypeDriverAdmin {
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
	return nil
}

func (b *Bot) notifyUser(dbID int64, text string) {
	target := b
	if b.Type != BotTypeClient && b.Peer != nil {
		target = b.Peer
	}
	var teleID int64
	b.DB.QueryRow(context.Background(), "SELECT telegram_id FROM users WHERE id=$1", dbID).Scan(&teleID)
	if teleID != 0 {
		target.Bot.Send(&tele.User{ID: teleID}, text, tele.ModeHTML)
	}
}

func (b *Bot) notifyDrivers(orderID, fromID, toID, tariffID int64, text string) {
	target := b
	if b.Type != BotTypeDriverAdmin && b.Peer != nil {
		target = b.Peer
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
		if u.Role != "driver" && u.Role != "admin" {
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

	// Add all days of the month
	for day := 1; day <= lastDay.Day(); day++ {
		dayDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

		// Disable past dates
		if dayDate.Before(time.Now().Truncate(24 * time.Hour)) {
			currentRow = append(currentRow, menu.Data(fmt.Sprintf("%d", day), "ignore"))
		} else {
			currentRow = append(currentRow, menu.Data(fmt.Sprintf("%d", day), fmt.Sprintf("%s%s", prefix, dayDate.Format("2006-01-02"))))
		}

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
		return c.Edit(header, menu)
	}
	return c.Send(header, menu)
}

func (b *Bot) getCurrentUser(c tele.Context) *models.User {
	u, _ := b.Stg.User().Get(context.Background(), c.Sender().ID)
	return u
}
