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
		return c.Send("❌ Извините, этот заказ уже принят или отменен.")
	}

	// 2. Atomically request the order (active -> wait_confirm + driver_id)
	err := b.Stg.Order().RequestOrder(context.Background(), id, dbID)
	if err != nil {
		return c.Send("❌ Xatolik: " + err.Error())
	}

	c.Send("⏳ So'rovingiz adminga yuborildi. Admin tasdiqlashini kuting...")

	// 3. Notify Admin
	driver, _ := b.Stg.User().Get(context.Background(), dbID)
	if driver == nil {
		b.Log.Error("Driver not found for notification", logger.Int64("driver_id", dbID))
		return c.Send("❌ Haydovchi ma'lumotlari topilmadi.")
	}

	phone := "Noma'lum"
	if driver.Phone != nil {
		phone = *driver.Phone
	}

	msg := fmt.Sprintf("🔔 <b>ВОДИТЕЛЬ ХОЧЕТ ПРИНЯТЬ ЗАКАЗ</b>\n\n🆔 Заказ: #%d\n🚖 Водитель: <a href=\"tg://user?id=%d\">%s</a>\n📞 Тел: %s\n\n👤 Клиент: <a href=\"tg://user?id=%d\">%s</a>\n📞 Тел: %s",
		id, driver.TelegramID, driver.FullName, phone, order.ClientID, order.ClientUsername, order.ClientPhone)

	b.notifyAdmin(id, msg, "match") // "match" type allows us to send specific buttons

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
	"ru": {
		"welcome":       "👋 Здравствуйте! Добро пожаловать в систему.",
		"contact_msg":   "Для регистрации, пожалуйста, отправьте ваш номер телефона:",
		"share_contact": "📱 Поделиться номером",
		"registered":    "🎉 Вы успешно зарегистрированы!",
		"blocked":       "🚫 Ваш аккаунт заблокирован.",
		"no_entry":      "🚫 Этот бот только для водителей и админов.",
		"menu_client":   "👤 Меню клиента:",
		"menu_driver":   "🚖 Меню водителя:",
		"menu_admin":    "🛠 Панель администратора:",
		"order_from":    "📍 Откуда вас забрать? (Город/район)",
		"order_to":      "🏁 Куда вы едете? (Город/район)",
		"order_tariff":  "🚕 Выберите тариф:",
		"order_pass":    "👥 Введите количество пассажиров:",
		"order_time":    "📅 Введите дату и время (Например: Сегодня 18:00):",
		"order_confirm": "💰 Детали заказа:\nЦена: %d %s\n\nПодтверждаете?",
		"order_created": "✅ Ваш заказ принят!",
		"no_orders":     "📭 На данный момент активных заказов нет.",
		"notif_new":     "🔔 НОВЫЙ ЗАКАЗ!\n🆔 #%d\n💰 Цена: %s\n📍 Маршрут: %s",
		"notif_taken":   "🚖 Ваш заказ принят водителем!\n\n🆔 ID: #%d\n🚗 Водитель: %s\n📞 Тел: %s\n👤 Профиль: %s",
		"notif_done":    "🏁 Ваш заказ успешно завершен. Спасибо!",
		"notif_cancel":  "⚠️ Заказ #%d отменен.",
		"help_client":   "📖 <b>Помощь для клиентов:</b>\n\n➕ <b>Создать заказ</b> - Создание нового заказа. Выберите город, напишите пункт назначения и выберите тариф.\n📋 <b>Мои заказы</b> - Все ваши заказы и их статус.\n🏠 <b>Главное меню</b> - Вернуться на главную страницу.",
		"help_driver":   "📖 <b>Помощь для водителей:</b>\n\n📦 <b>Активные заказы</b> - Список всех свободных заказов на данный момент.\n📍 <b>Мои маршруты</b> - Города, по которым вы работаете. Уведомления приходят только по этим маршрутам.\n🚕 <b>Мои тарифы</b> - Тарифы, по которым вы работаете (Эконом, Комфорт и т.д.).\n📅 <b>Поиск по дате</b> - Просмотр заказов на определенную дату.\n📋 <b>Мои заказы</b> - Заказы, которые вы приняли и выполняете.",
		"help_admin":    "📖 <b>Помощь админ-панели:</b>\n\n👥 <b>Пользователи</b> - Управление пользователями (роли, блокировка).\n📦 <b>Все заказы</b> - История всех заказов в системе.\n⚙️ <b>Тарифы</b> - Добавление или удаление тарифов.\n🗺 <b>Города</b> - Редактирование списка городов.\n📊 <b>Статистика</b> - Общая статистика использования.",
	},
}

func (b *Bot) registerHandlers() {
	b.Bot.Handle("/start", b.handleStart)
	b.Bot.Handle("/help", b.handleHelp)

	// Client Handlers
	if b.Type == BotTypeClient {
		b.Bot.Handle(tele.OnContact, b.handleContact)
		b.Bot.Handle("➕ Создать заказ", b.handleOrderStart)
		b.Bot.Handle("📋 Мои заказы", b.handleMyOrders)
		b.Bot.Handle("🏠 Главное меню", b.handleStart)
	}

	// Driver Handlers
	if b.Type == BotTypeDriver {
		b.Bot.Handle(tele.OnContact, b.handleContact)
		b.Bot.Handle("📦 Активные заказы", b.handleActiveOrders)
		b.Bot.Handle("📋 Мои заказы", b.handleMyOrdersDriver)
		b.Bot.Handle("📍 Мои маршруты", b.handleDriverRoutes)
		b.Bot.Handle("🚕 Мои тарифы", b.handleDriverTariffs)
		b.Bot.Handle("Поиск по дате", b.handleDriverCalendarSearch)
		b.Bot.Handle("🏠 Главное меню", b.handleStart)
	}

	// Admin Handlers
	if b.Type == BotTypeAdmin {
		b.Bot.Handle(tele.OnContact, b.handleContact)
		b.Bot.Handle("👥 Пользователи", b.handleAdminUsers)
		b.Bot.Handle("📦 Все заказы", b.handleAdminOrders)
		b.Bot.Handle("⚙️ Тарифы", b.handleAdminTariffs)
		b.Bot.Handle("🗺 Города", b.handleAdminLocations)
		b.Bot.Handle("📊 Статистика", b.handleAdminStats)
		b.Bot.Handle("➕ Добавить тариф", b.handleTariffAddStart)
		b.Bot.Handle("🗑 Удалить по ID", b.handleTariffDeleteStart)
		b.Bot.Handle("➕ Добавить город", b.handleLocationAddStart)
		b.Bot.Handle("🗑 Удалить по ID", b.handleLocationDeleteStart)
		b.Bot.Handle("🔍 Получить по ID", b.handleLocationGetStart)
		b.Bot.Handle("🏠 Главное меню", b.handleStart)
	}

	b.Bot.Handle(tele.OnCallback, b.handleCallback)
	b.Bot.Handle(tele.OnText, b.handleText)
	b.Bot.Handle(tele.OnWebApp, b.handleWebApp)

	// Set Bot Commands for UI hint
	cmds := []tele.Command{
		{Text: "start", Description: "Запустить бота / Главное меню"},
		{Text: "help", Description: "Помощь и руководство"},
	}
	b.Bot.SetCommands(cmds)
}

func (b *Bot) handleAdminLogin(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		b.Sessions[c.Sender().ID] = &UserSession{State: "awaiting_login"}
		session = b.Sessions[c.Sender().ID]
	}

	if session.State == "awaiting_login" {
		session.State = "awaiting_login_input"
		session.LastActionTime = time.Now() // timeout ni oldini olish uchun
		return c.Send("🔐 <b>Админ-система</b>\n\nВведите логин:", tele.ModeHTML)
	}

	if session.State == "awaiting_login_input" {
		session.TempString = c.Text() // login ni saqlash uchun
		session.State = "awaiting_password"
		session.LastActionTime = time.Now() // timeout ni oldini olish uchun
		return c.Send("🔑 Введите пароль:", tele.ModeHTML)
	}

	if session.State == "awaiting_password" {
		login := session.TempString
		password := c.Text()

		if login == b.Cfg.AdminLogin && password == b.Cfg.AdminPassword {
			session.State = "authenticated"
			session.LastActionTime = time.Now() // timeout ni oldini olish uchun
			return c.Send("✅ Успешный вход! Меню администратора:", tele.ModeHTML)
		} else {
			session.State = "awaiting_login"
			session.TempString = ""
			session.LastActionTime = time.Now()
			return c.Send("❌ Логин или пароль неверны!\n\nПопробуйте еще раз:", tele.ModeHTML)
		}
	}

	return nil
}

func (b *Bot) handleStart(c tele.Context) error {
	b.Log.Info(fmt.Sprintf("Start command received from %d (%s)", c.Sender().ID, c.Sender().Username))

	// Admin bot uchun login/parol tekshiruvi
	if b.Type == BotTypeAdmin {
		session := b.Sessions[c.Sender().ID]
		if session == nil || session.State != "authenticated" {
			return b.handleAdminLogin(c)
		}
	}

	ctx := context.Background()
	user, _ := b.Stg.User().GetOrCreate(ctx, c.Sender().ID, c.Sender().Username, fmt.Sprintf("%s %s", c.Sender().FirstName, c.Sender().LastName))

	isAdmin := (b.Cfg.AdminID != 0 && c.Sender().ID == b.Cfg.AdminID) ||
		(b.Cfg.AdminUsername != "" && c.Sender().Username == b.Cfg.AdminUsername)

	if isAdmin && user.Role != "admin" {
		b.Stg.User().UpdateRole(ctx, c.Sender().ID, "admin")
		user, _ = b.Stg.User().Get(ctx, c.Sender().ID)
	}

	if (b.Type == BotTypeDriver || b.Type == BotTypeAdmin) && !isAdmin && user.Role == "client" && user.Status != "pending" {
		return c.Send("🚫 <b>Доступ запрещен!</b>\n\nВы зарегистрированы как клиент.\n\n👇 Пожалуйста, перейдите в бот для клиентов:\n@clienttaxigo_bot", tele.ModeHTML)
	}

	if b.Type == BotTypeClient && user.Role == "driver" {
		return c.Send("🚫 <b>Доступ запрещен!</b>\n\nВы зарегистрированы как водитель.\n\n👇 Пожалуйста, перейдите в бот для водителей:\n@drivertaxisgo_bot", tele.ModeHTML)
	}

	// Always initialize/reset session on /start
	b.Sessions[c.Sender().ID] = &UserSession{
		DBID:      user.ID,
		State:     StateIdle,
		OrderData: &models.Order{ClientID: user.ID},
	}

	if user.Status == "pending" {
		menu := &tele.ReplyMarkup{ResizeKeyboard: true}
		menu.Reply(menu.Row(menu.Contact(messages["ru"]["share_contact"])))
		return c.Send(messages["ru"]["contact_msg"], menu)
	}

	return b.showMenu(c, user)
}

func (b *Bot) handleContact(c tele.Context) error {
	b.Log.Info(fmt.Sprintf("Contact received from %d", c.Sender().ID))
	if c.Message().Contact.UserID != c.Sender().ID {
		return c.Send("Пожалуйста, отправьте свой собственный номер.")
	}
	ctx := context.Background()
	b.Stg.User().UpdatePhone(ctx, c.Sender().ID, c.Message().Contact.PhoneNumber)

	// If registering via Driver Bot, set role to driver
	if b.Type == BotTypeDriver {
		b.Stg.User().UpdateRole(ctx, c.Sender().ID, "driver")
	}

	b.Stg.User().UpdateStatus(ctx, c.Sender().ID, "active")
	user, _ := b.Stg.User().Get(ctx, c.Sender().ID)

	c.Send(messages["ru"]["registered"], tele.RemoveKeyboard)

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
			menu.Row(menu.Text("➕ Создать заказ")),
			menu.Row(menu.Text("📋 Мои заказы")),
			menu.Row(menu.Text("🏠 Главное меню")),
		)
		return c.Send(messages["ru"]["menu_client"], &tele.SendOptions{ReplyMarkup: menu})
	}

	if user.Role == "admin" {
		menu.Reply(
			menu.Row(menu.Text("👥 Пользователи"), menu.Text("📦 Все заказы")),
			menu.Row(menu.Text("⚙️ Тарифы"), menu.Text("🗺 Города")),
			menu.Row(menu.Text("📊 Статистика")),
			menu.Row(menu.Text("📦 Активные заказы"), menu.Text("📋 Мои заказы")),
			menu.Row(menu.Text("🏠 Главное меню")),
		)
		return c.Send(messages["ru"]["menu_admin"], &tele.SendOptions{ReplyMarkup: menu})
	}

	// Driver Menu
	menu.Reply(
		menu.Row(menu.Text("📦 Активные заказы")),
		menu.Row(menu.Text("📍 Мои маршруты"), menu.Text("🚕 Мои тарифы")),
		menu.Row(menu.Text("Поиск по дате")),
		menu.Row(menu.Text("📋 Мои заказы")),
	)
	return c.Send(messages["ru"]["menu_driver"], &tele.SendOptions{ReplyMarkup: menu})
}

func (b *Bot) handleHelp(c tele.Context) error {
	user := b.getCurrentUser(c)
	if user == nil {
		return c.Send("📖 Для использования системы нажмите /start.")
	}

	msgKey := "help_client"
	if user.Role == "admin" && b.Type == BotTypeAdmin {
		msgKey = "help_admin"
	} else if user.Role == "driver" || b.Type == BotTypeDriver {
		msgKey = "help_driver"
	}

	return c.Send(messages["ru"][msgKey], tele.ModeHTML)
}

func (b *Bot) handleOrderStart(c tele.Context) error {
	b.Log.Info("DEBUG: Handle Order Start", logger.Int64("user_id", c.Sender().ID))
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
		}
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
	return c.Send(messages["ru"]["order_from"], menu, tele.ModeHTML)
}

func (b *Bot) handleActiveOrders(c tele.Context) error {
	orders, _ := b.Stg.Order().GetActiveOrders(context.Background())
	if len(orders) == 0 {
		return c.Send(messages["ru"]["no_orders"])
	}

	for _, o := range orders {
		timeStr := "Неизвестно"
		if o.PickupTime != nil {
			timeStr = o.PickupTime.Format("02.01.2006 15:04")
		}

		txt := fmt.Sprintf("📦 <b>НОВЫЙ ЗАКАЗ #%d</b>\n\n📍 Маршрут: <b>%s ➡️ %s</b>\n💰 Цена: <b>%d %s</b>\n👥 Пассажиры: <b>%d</b>\n🕒 Время: <b>%s</b>\n\n👤 Клиент: <a href=\"tg://user?id=%d\">%s</a>\n📞 Тел: %s",
			o.ID, o.FromLocationName, o.ToLocationName, o.Price, o.Currency, o.Passengers, timeStr, o.ClientID, o.ClientUsername, o.ClientPhone)

		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(menu.Data("📥 Принять заказ", fmt.Sprintf("take_%d", o.ID))))
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
		return c.Send("У вас нет принятых заказов.")
	}

	for _, o := range orders {
		timeStr := "Неизвестно"
		if o.PickupTime != nil {
			timeStr = o.PickupTime.Format("02.01.2006 15:04")
		}

		txt := fmt.Sprintf("🚖 <b>ЗАКАЗ #%d</b>\n📍 %s ➡️ %s\n👥 Пассажиры: %d\n💰 Цена: %d %s\n📅 Время: %s\n📊 Статус: %s\n\n👤 Клиент: <a href=\"tg://user?id=%d\">%s</a>\n📞 Тел: %s",
			o.ID, o.FromLocationName, o.ToLocationName, o.Passengers, o.Price, o.Currency, timeStr, o.Status, o.ClientID, o.ClientUsername, o.ClientPhone)

		menu := &tele.ReplyMarkup{}
		if o.Status == "taken" {
			menu.Inline(menu.Row(menu.Data("✅ Завершить", fmt.Sprintf("complete_%d", o.ID))))
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
		return c.Send("👥 Пользователи не найдены.")
	}

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("👥 <b>Список пользователей (%d/%d):</b>\n\n", page+1, totalPages))

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	for i := start; i < end; i++ {
		u := users[i]
		phone := "Неизвестно"
		if u.Phone != nil {
			phone = *u.Phone
		}

		msg.WriteString(fmt.Sprintf("🆔 <b>%d</b> | %s\n📞 %s | Роль: <b>%s</b> | Статус: <b>%s</b>\n", u.TelegramID, u.FullName, phone, u.Role, u.Status))
		msg.WriteString("------------------------------\n")

		btnRole := menu.Data(fmt.Sprintf("🔄 Роль (%s)", u.FullName), fmt.Sprintf("adm_role_%d_%d", u.TelegramID, page))
		btnStatus := menu.Data(fmt.Sprintf("🚫/✅ (%s)", u.FullName), fmt.Sprintf("adm_stat_%d_%d", u.TelegramID, page))
		rows = append(rows, menu.Row(btnRole, btnStatus))
	}

	// Navigation
	var navRow []tele.Btn
	if page > 0 {
		navRow = append(navRow, menu.Data("⬅️ Предыдущая", fmt.Sprintf("users_page_%d", page-1)))
	}
	if page < totalPages-1 {
		navRow = append(navRow, menu.Data("Следующая ➡️", fmt.Sprintf("users_page_%d", page+1)))
	}
	// Always add Back button
	navRow = append(navRow, menu.Data("⬅️ Назад", "admin_back"))

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
		return c.Send("📦 Заказы не найдены.")
	}

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("📦 <b>Все заказы (%d/%d):</b>\n\n", page+1, totalPages))

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	for i := start; i < end; i++ {
		o := orders[i]
		msg.WriteString(fmt.Sprintf("🔹 <b>#%d</b> | %s\n📍 %s -> %s\n💰 %d %s\n\n", o.ID, o.Status, o.FromLocationName, o.ToLocationName, o.Price, o.Currency))

		if o.Status != "completed" && o.Status != "cancelled" {
			rows = append(rows, menu.Row(menu.Data(fmt.Sprintf("❌ Отменить #%d", o.ID), fmt.Sprintf("adm_cancel_%d_%d", o.ID, page))))
		}
	}

	var navRow []tele.Btn
	if page > 0 {
		navRow = append(navRow, menu.Data("⬅️ Предыдущая", fmt.Sprintf("orders_page_%d", page-1)))
	}
	if page < totalPages-1 {
		navRow = append(navRow, menu.Data("Следующая ➡️", fmt.Sprintf("orders_page_%d", page+1)))
	}
	// Always add Back button
	navRow = append(navRow, menu.Data("⬅️ Назад", "admin_back"))

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
	menu.Reply(
		menu.Row(menu.Text("➕ Добавить тариф"), menu.Text("🗑 Удалить по ID")),
		menu.Row(menu.Text("🏠 Главное меню")),
	)

	var msg strings.Builder
	msg.WriteString("⚙️ <b>Доступные тарифы:</b>\n\n")
	for i, t := range tariffs {
		msg.WriteString(fmt.Sprintf("%d. 🚕 <b>%s</b> (ID: %d)\n", i+1, t.Name, t.ID))
	}

	return c.Send(msg.String(), menu, tele.ModeHTML)
}

func (b *Bot) handleTariffDeleteStart(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
		}
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}

	session.State = "awaiting_tariff_delete_id"

	return c.Send("🗑 <b>Удаление тарифа</b>\n\nВведите ID тарифа, который хотите удалить:", tele.ModeHTML)
}

func (b *Bot) handleLocationDeleteStart(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
		}
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}

	session.State = "awaiting_location_delete_id"

	return c.Send("🗑 <b>Удаление города</b>\n\nВведите ID города, который хотите удалить:", tele.ModeHTML)
}

func (b *Bot) handleLocationGetStart(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	if session == nil {
		user := b.getCurrentUser(c)
		if user == nil {
			return c.Send("❌ Ошибка: Информация о пользователе не найдена. Пожалуйста, нажмите /start еще раз.")
		}
		b.Sessions[c.Sender().ID] = &UserSession{DBID: user.ID, State: StateIdle}
		session = b.Sessions[c.Sender().ID]
	}

	session.State = "awaiting_location_get_id"

	return c.Send("🔍 <b>Получить город</b>\n\nВведите ID города для поиска:", tele.ModeHTML)
}

func (b *Bot) handleAdminLocations(c tele.Context) error {
	locations, _ := b.Stg.Location().GetAll(context.Background())
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(
		menu.Row(menu.Text("➕ Добавить город"), menu.Text("🗑 Удалить по ID")),
		menu.Row(menu.Text("🔍 Получить по ID")),
		menu.Row(menu.Text("🏠 Главное меню")),
	)

	var msg strings.Builder
	msg.WriteString("🗺 <b>Доступные города:</b>\n\n")
	msg.WriteString("┌─────┬──────────────────────┐\n")
	msg.WriteString("│  ID │       Название       │\n")
	msg.WriteString("├─────┼──────────────────────┤\n")

	for _, l := range locations {
		msg.WriteString(fmt.Sprintf("│ %4d │ %-20s │\n", l.ID, l.Name))
	}

	msg.WriteString("└─────┴──────────────────────┘\n")

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
	return c.Send(fmt.Sprintf("📊 СТАТИСТИКА\n\nВсего пользователей: %d\nАктивные заказы: %d\nВсего заказов: %d", len(users), active, len(orders)))
}

func (b *Bot) handleText(c tele.Context) error {
	b.Log.Info("Handle Text", logger.String("text", c.Text()))

	session, ok := b.Sessions[c.Sender().ID]
	if !ok || session.State == StateIdle {
		return nil
	}

	// Guard: If it's a menu button text, don't process it as input for states
	txt := c.Text()
	isMenu := txt == "➕ Создать заказ" || txt == "📋 Мои заказы" || txt == "🏠 Главное меню" ||
		txt == "📦 Активные заказы" || txt == "📍 Мои маршруты" || txt == "🚕 Мои тарифы" ||
		txt == "Поиск по дате" || txt == "👥 Пользователи" || txt == "📦 Все заказы" ||
		txt == "⚙️ Тарифы" || txt == "🗺 Города" || txt == "📊 Статистика" ||
		txt == "➕ Добавить тариф" || txt == "🗑 Удалить по ID" ||
		txt == "➕ Добавить город" || txt == "🔍 Получить по ID"

	if isMenu {
		// If it's a menu button, we should probably reset state and let the specific handler take over
		// But handlers for these are already registered, so we just return nil here to stop handleText
		return nil
	}

	b.Log.Info("Processing Text State", logger.String("state", session.State))

	// Admin login/parol uchun
	if session.State == "awaiting_login" {
		session.State = "awaiting_login_input"
		session.LastActionTime = time.Now()
		return c.Send("🔐 <b>Админ-система</b>\n\nВведите логин:", tele.ModeHTML)
	}

	if session.State == "awaiting_login_input" {
		session.TempString = c.Text() // login ni saqlaymiz
		session.State = "awaiting_password"
		session.LastActionTime = time.Now()
		return c.Send("🔑 Введите пароль:", tele.ModeHTML)
	}

	if session.State == "awaiting_password" {
		return b.handleAdminLogin(c)
	}

	switch session.State {
	case StateFrom:
		session.TempString = c.Text()
		session.State = StateTo
		return c.Send(messages["ru"]["order_to"])
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
		return c.Send(messages["ru"]["order_tariff"], menu)
	case StateTariffAdd:
		b.Stg.Tariff().Create(context.Background(), c.Text())
		session.State = StateIdle
		return b.showMenu(c, b.getCurrentUser(c))
	case StateLocationAdd:
		b.Stg.Location().Create(context.Background(), c.Text())
		session.State = StateIdle
		return b.showMenu(c, b.getCurrentUser(c))
	case "awaiting_tariff_delete_id":
		id, err := strconv.ParseInt(c.Text(), 10, 64)
		if err != nil {
			return c.Send("❌ Неверный ID! Пожалуйста, введите число.")
		}
		err = b.Stg.Tariff().Delete(context.Background(), id)
		if err != nil {
			return c.Send("❌ Ошибка: " + err.Error())
		}
		session.State = StateIdle
		return c.Send("✅ Тариф успешно удален!")
	case "awaiting_location_delete_id":
		id, err := strconv.ParseInt(c.Text(), 10, 64)
		if err != nil {
			return c.Send("❌ Неверный ID! Пожалуйста, введите число.")
		}
		err = b.Stg.Location().Delete(context.Background(), id)
		if err != nil {
			return c.Send("❌ Ошибка: " + err.Error())
		}
		session.State = StateIdle
		return c.Send("✅ Город успешно удален!")
	case "awaiting_location_get_id":
		id, err := strconv.ParseInt(c.Text(), 10, 64)
		if err != nil {
			return c.Send("❌ Неверный ID! Пожалуйста, введите число.")
		}
		location, err := b.Stg.Location().GetByID(context.Background(), id)
		if err != nil {
			return c.Send("❌ Город не найден!")
		}
		session.State = StateIdle
		return c.Send(fmt.Sprintf("🔍 <b>Информация о городе:</b>\n\n🆔 ID: %d\n📍 Название: %s", location.ID, location.Name), tele.ModeHTML)
	case StateAdminLogin:
		if c.Text() == "zarif" {
			session.State = StateAdminPassword
			return c.Send("🔑 Введите пароль:")
		} else {
			return c.Send("❌ Ошибка логина! Введите еще раз:")
		}
	case StateAdminPassword:
		if c.Text() == "1234" {
			// Success
			b.Stg.User().UpdateRole(context.Background(), session.DBID, "admin")
			session.State = StateIdle
			user, _ := b.Stg.User().Get(context.Background(), c.Sender().ID)
			c.Send("✅ Успешный вход!")
			return b.showMenu(c, user)
		} else {
			return c.Send("❌ Ошибка пароля! Введите еще раз:")
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

	b.Log.Info("DEBUG: Handle Callback",
		logger.String("data", data),
		logger.Bool("is_order_flow", isOrderFlowCallback),
		logger.Int64("from_id", session.OrderData.FromLocationID),
		logger.Int64("to_id", session.OrderData.ToLocationID),
		logger.Int64("tariff_id", session.OrderData.TariffID),
	)

	if isOrderFlowCallback && session.OrderData.FromLocationID == 0 {

		c.Delete() // Delete the stale message/keyboard
		return c.Send("⚠️ <b>Сессия обновлена.</b>\n\nИз-за перезапуска бота, пожалуйста, оформите заказ заново:\n/start", tele.ModeHTML)
	}

	// Guard: Check for ToLocationID for steps that require it
	if (strings.HasPrefix(data, "tf_") ||
		strings.HasPrefix(data, "cal_") ||
		strings.HasPrefix(data, "time_") ||
		strings.HasPrefix(data, "confirm_")) && session.OrderData.ToLocationID == 0 {

		c.Delete()
		return c.Send("⚠️ <b>Сессия обновлена.</b>\n\nИз-за перезапуска бота, пожалуйста, оформите заказ заново:\n/start", tele.ModeHTML)
	}

	// Guard: Check for TariffID for steps that require it
	if (strings.HasPrefix(data, "cal_") ||
		strings.HasPrefix(data, "time_") ||
		strings.HasPrefix(data, "confirm_")) && session.OrderData.TariffID == 0 {
		c.Delete()
		return c.Send("⚠️ <b>Сессия обновлена.</b>\n\nИз-за перезапуска бота, пожалуйста, оформите заказ заново:\n/start", tele.ModeHTML)
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
		return c.Edit(messages["ru"]["order_to"], menu, tele.ModeHTML)
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

		fromName := "Неизвестно"
		if from != nil {
			fromName = from.Name
		}
		toName := "Неизвестно"
		if to != nil {
			toName = to.Name
		}

		session.TempString = fmt.Sprintf("%s ➡️ %s", fromName, toName)
		return c.Edit(messages["ru"]["order_tariff"], menu)
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
		return c.Edit("🕒 Выберите время:", menu)
	}

	if strings.HasPrefix(data, "take_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "take_"), 10, 64)
		b.Bot.Edit(c.Callback().Message, "✅ Заказ принят!")
		return b.handleTakeOrderWithID(c, id)
	}

	if strings.HasPrefix(data, "complete_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "complete_"), 10, 64)
		b.Stg.Order().CompleteOrder(context.Background(), id)
		b.Bot.Edit(c.Callback().Message, "🏁 Заказ завершен!")
		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order != nil {
			b.notifyUser(order.ClientID, messages["ru"]["notif_done"])
		}
		return c.Respond()
	}

	if strings.HasPrefix(data, "cancel_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "cancel_"), 10, 64)
		b.Stg.Order().CancelOrder(context.Background(), id)
		b.Bot.Edit(c.Callback().Message, "❌ Отменено.")
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
		return c.Edit("<b>🏁 Куда вы едете?</b>\nВыберите город:", tele.ModeHTML)
	}

	if strings.HasPrefix(data, "dr_t_") {
		toID, _ := strconv.ParseInt(strings.TrimPrefix(data, "dr_t_"), 10, 64)
		fromID := session.OrderData.FromLocationID

		b.Stg.Route().AddRoute(context.Background(), session.DBID, fromID, toID)
		c.Respond(&tele.CallbackResponse{Text: "Маршрут добавлен!"})
		return b.handleDriverRoutes(c)
	}

	if strings.HasPrefix(data, "tgl_") {
		tariffID, _ := strconv.ParseInt(strings.TrimPrefix(data, "tgl_"), 10, 64)
		b.Stg.Tariff().Toggle(context.Background(), session.DBID, tariffID)
		return b.showDriverTariffs(c, false)
	}

	if data == "tf_del_mode" {
		return b.showDriverTariffs(c, true)
	}

	if data == "tf_back" {
		return b.showDriverTariffs(c, false)
	}

	if strings.HasPrefix(data, "del_tf_") {
		tariffID, _ := strconv.ParseInt(strings.TrimPrefix(data, "del_tf_"), 10, 64)
		b.Stg.Tariff().Delete(context.Background(), tariffID)
		return b.showDriverTariffs(c, true)
	}

	switch data {
	case "confirm_yes":
		if session.OrderData == nil || session.OrderData.FromLocationID == 0 || session.OrderData.ToLocationID == 0 || session.OrderData.TariffID == 0 {
			b.Log.Warning("Invalid order data in session for confirm_yes", logger.Int64("user_id", c.Sender().ID))
			return c.Send("⚠️ <b>Ошибка:</b> Данные заказа не найдены. Пожалуйста, нажмите /start и оформите заказ заново.", tele.ModeHTML)
		}

		// Get client info for order
		client, _ := b.Stg.User().GetByID(context.Background(), session.DBID)
		if client != nil {
			if client.Username != "" {
				session.OrderData.ClientUsername = client.Username
			} else {
				session.OrderData.ClientUsername = "Неизвестно"
			}
			if client.Phone != nil {
				session.OrderData.ClientPhone = *client.Phone
			} else {
				session.OrderData.ClientPhone = "Неизвестно"
			}
		} else {
			session.OrderData.ClientUsername = "Неизвестно"
			session.OrderData.ClientPhone = "Неизвестно"
		}

		session.OrderData.Status = "pending"
		order, err := b.Stg.Order().Create(context.Background(), session.OrderData)
		if err == nil {
			c.Send(messages["ru"]["order_created"])
			// New Flow: Notify Admin for approval
			adminMsg := fmt.Sprintf("🔔 <b>НОВЫЙ ЗАКАЗ (На утверждение)</b>\n\n🆔 #%d\n📍 %s ➡️ %s\n💰 %d %s\n👥 %d пассажиров\n📅 %s",
				order.ID, session.TempString, c.Text(), order.Price, order.Currency, order.Passengers, session.TempString)
			// Note: TempString might be overwritten or not perfect, ideally reconstruct from IDs.
			// Reconstructing strictly for Admin message:
			from, _ := b.Stg.Location().GetByID(context.Background(), session.OrderData.FromLocationID)
			to, _ := b.Stg.Location().GetByID(context.Background(), session.OrderData.ToLocationID)
			fromName, toName := "Неизвестно", "Неизвестно"
			if from != nil {
				fromName = from.Name
			}
			if to != nil {
				toName = to.Name
			}
			timeStr := "Сейчас"
			if session.OrderData.PickupTime != nil {
				timeStr = session.OrderData.PickupTime.Format("02.01.2006 15:04")
			}

			clientName := "Неизвестно"
			clientTeleID := int64(0)
			if client != nil {
				clientName = client.FullName
				clientTeleID = client.TelegramID
			}

			adminMsg = fmt.Sprintf("🔔 <b>НОВЫЙ ЗАКАЗ (На утверждение)</b>\n\n🆔 #%d\n📍 %s ➡️ %s\n💰 Цена: %d %s\n👥 Пассажиры: %d\n📅 Время: %s\n\n👤 Клиент: <a href=\"tg://user?id=%d\">%s</a>\n📞 Тел: %s",
				order.ID, fromName, toName, order.Price, order.Currency, order.Passengers, timeStr, clientTeleID, clientName, order.ClientPhone)

			b.notifyAdmin(order.ID, adminMsg)
			c.Send("⏳ Ваш заказ отправлен администратору. Ожидайте подтверждения.")
		} else {
			b.Log.Error("Order creation failed", logger.Error(err))
			c.Send("❌ Произошла ошибка при создании заказа.")
		}
		session.State = StateIdle
		return b.showMenu(c, b.getCurrentUser(c))
	case "confirm_no":
		session.State = StateIdle
		c.Send("❌ Отменено.")
		return b.showMenu(c, b.getCurrentUser(c))
	}

	if data == "ignore" {
		return c.Respond(&tele.CallbackResponse{Text: ""})
	}

	if strings.HasPrefix(data, "time_") {
		timeStr := strings.TrimPrefix(data, "time_") // "14:00"
		if session.TempString == "" {
			c.Delete()
			return c.Send("⚠️ <b>Ошибка:</b> Дата не выбрана. Пожалуйста, нажмите /start и оформите заказ заново.", tele.ModeHTML)
		}

		fullTimeStr := fmt.Sprintf("%s %s", session.TempString, timeStr) // "2023-10-27 14:00"
		loc := time.FixedZone("Europe/Moscow", 3*60*60)
		parsedTime, err := time.ParseInLocation("2006-01-02 15:04", fullTimeStr, loc)
		if err != nil {
			b.Log.Error("Failed to parse time", logger.Error(err), logger.String("fullTimeStr", fullTimeStr))
			c.Delete()
			return c.Send("⚠️ <b>Ошибка:</b> Неверный формат времени. Пожалуйста, нажмите /start и оформите заказ заново.", tele.ModeHTML)
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

		fromName := "Неизвестно"
		if from != nil {
			fromName = from.Name
		}
		toName := "Неизвестно"
		if to != nil {
			toName = to.Name
		}
		tariffName := "Неизвестно"
		if tariff != nil {
			tariffName = tariff.Name
		}

		msg := fmt.Sprintf("<b>💰 Подтверждение заказа</b>\n\n📍 <b>%s ➡️ %s</b>\n🚕 Тариф: <b>%s</b>\n👥 Пассажиры: <b>%d</b>\n📅 Время: <b>%s</b>\n\nПодтверждаете?",
			fromName, toName, tariffName, session.OrderData.Passengers, parsedTime.Format("02.01.2006 15:04"))

		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(
			menu.Data("✅ Подтвердить", "confirm_yes"),
			menu.Data("❌ Отменить", "confirm_no"),
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
		return c.Respond(&tele.CallbackResponse{Text: "Заблокировано"})
	}
	if strings.HasPrefix(data, "user_act_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "user_act_"), 10, 64)
		b.Stg.User().UpdateStatus(context.Background(), id, "active")
		return c.Respond(&tele.CallbackResponse{Text: "Активировано"})
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
				from, _ := b.Stg.Location().GetByID(context.Background(), order.FromLocationID)
				to, _ := b.Stg.Location().GetByID(context.Background(), order.ToLocationID)
				fromName, toName := "Неизвестно", "Неизвестно"
				if from != nil {
					fromName = from.Name
				}
				if to != nil {
					toName = to.Name
				}
				priceStr := fmt.Sprintf("%d %s", order.Price, order.Currency)

				notifMsg := fmt.Sprintf(messages["ru"]["notif_new"], order.ID, priceStr, fromName, toName)

				b.notifyDrivers(order.ID, order.FromLocationID, order.ToLocationID, order.TariffID, notifMsg)

				// Notify Client
				b.notifyUser(order.ClientID, "✅ Ваш заказ подтвержден администратором! Ищем водителя...")
				return c.Edit("✅ Подтверждено и отправлено водителям.")
			}
		}
		return c.Edit("❌ Произошла ошибка.")
	}

	if strings.HasPrefix(data, "reject_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "reject_"), 10, 64)
		b.Stg.Order().CancelOrder(context.Background(), id)
		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order != nil {
			b.notifyUser(order.ClientID, "❌ Ваш заказ отменен администратором.")
		}
		return c.Edit("❌ Отклонено.")
	}

	// Match Approval (Driver <-> Client)
	if strings.HasPrefix(data, "approve_match_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "approve_match_"), 10, 64)

		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order == nil {
			return c.Edit("❌ Заказ не найден.")
		}
		if order.Status != "wait_confirm" || order.DriverID == nil {
			return c.Edit("❌ Этот заказ не находится в статусе ожидания подтверждения.")
		}

		// 1. Finalize Order (wait_confirm -> taken)
		if _, err := b.DB.Exec(context.Background(), "UPDATE orders SET status='taken' WHERE id=$1 AND status='wait_confirm'", id); err != nil {
			return c.Edit("❌ Произошла ошибка.")
		}

		// 2. Notify Client (with Driver details)
		driver, _ := b.Stg.User().GetByID(context.Background(), *order.DriverID)
		phone := "Неизвестно"
		if driver != nil {
			if driver.Phone != nil {
				phone = *driver.Phone
			}
			profile := fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>", driver.TelegramID, driver.FullName)
			msg := fmt.Sprintf(messages["ru"]["notif_taken"], id, driver.FullName, phone, profile)
			b.notifyUser(order.ClientID, msg)
		}

		// 3. Notify Driver
		client, _ := b.Stg.User().GetByID(context.Background(), order.ClientID)
		clientInfo := "Данные клиента недоступны"
		if client != nil {
			clientPhone := "Неизвестно"
			if client.Phone != nil {
				clientPhone = *client.Phone
			}
			clientInfo = fmt.Sprintf("👤 Клиент: <a href=\"tg://user?id=%d\">%s</a>\n📞 Тел: %s", client.TelegramID, client.FullName, clientPhone)
		}
		b.notifyDriverSpecific(*order.DriverID, fmt.Sprintf("✅ Админ подтвердил заказ! (#%d)\n\n%s\n\nСвяжитесь с клиентом.", id, clientInfo))

		return c.Edit("✅ Успешно прикреплено.")
	}

	if strings.HasPrefix(data, "reject_match_") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(data, "reject_match_"), 10, 64)

		order, _ := b.Stg.Order().GetByID(context.Background(), id)
		if order == nil {
			return c.Edit("❌ Заказ не найден.")
		}
		var requestedDriverID *int64
		if order.DriverID != nil {
			tmp := *order.DriverID
			requestedDriverID = &tmp
		}

		// 1. Reset Status to Active only if still waiting confirm
		if _, err := b.DB.Exec(context.Background(), "UPDATE orders SET status='active', driver_id=NULL WHERE id=$1 AND status='wait_confirm'", id); err != nil {
			return c.Edit("❌ Произошла ошибка.")
		}

		// 2. Notify rejected driver
		if requestedDriverID != nil {
			b.notifyDriverSpecific(*requestedDriverID, fmt.Sprintf("❌ Админ отклонил ваш запрос на заказ. (#%d)", id))
		}

		return c.Edit("❌ Отклонено. Заказ снова активирован.")
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
			menu.Data("✅ Подтвердить", fmt.Sprintf("approve_match_%d", orderID)),
			menu.Data("❌ Отклонить", fmt.Sprintf("reject_match_%d", orderID)),
		))
	} else {
		menu.Inline(menu.Row(
			menu.Data("✅ Подтвердить", fmt.Sprintf("approve_%d", orderID)),
			menu.Data("❌ Отменить", fmt.Sprintf("reject_%d", orderID)),
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
		menu.Data("📥 Принять заказ", fmt.Sprintf("take_%d", orderID)),
		menu.Data("❌ Закрыть", "close_msg"),
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
		return c.Send("У вас нет заказов.")
	}
	for _, o := range orders {
		timeStr := "Неизвестно"
		if o.PickupTime != nil {
			timeStr = o.PickupTime.Format("02.01.2006 15:04")
		}

		txt := fmt.Sprintf("📦 <b>Заказ #%d</b>\n📍 %s ➡️ %s\n👥 Пассажиры: %d\n📅 Время: %s\n📊 Статус: %s",
			o.ID, o.FromLocationName, o.ToLocationName, o.Passengers, timeStr, o.Status)

		menu := &tele.ReplyMarkup{}
		if o.Status == "active" || o.Status == "pending" {
			menu.Inline(menu.Row(menu.Data("❌ Отменить", fmt.Sprintf("cancel_%d", o.ID))))
		}
		c.Send(txt, menu, tele.ModeHTML)
	}
	return nil
}

func (b *Bot) handleTariffAddStart(c tele.Context) error {
	b.Sessions[c.Sender().ID].State = StateTariffAdd
	return c.Send("Введите название тарифа:")
}

func (b *Bot) handleLocationAddStart(c tele.Context) error {
	b.Sessions[c.Sender().ID].State = StateLocationAdd
	return c.Send("Введите название города/района:")
}

func (b *Bot) generateCalendar(c tele.Context, year, month int) error {
	return b.generateCalendarWithPrefix(c, year, month, "cal_")
}

func (b *Bot) generateCalendarWithPrefix(c tele.Context, year, month int, prefix string) error {
	// Month names in Russian
	monthNames := []string{"", "Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
		"Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь"}

	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	// Header with month and year
	header := fmt.Sprintf("📅 %s %d", monthNames[month], year)

	// Week day names
	rows = append(rows, menu.Row(
		menu.Data("Пн", "ignore"), menu.Data("Вт", "ignore"), menu.Data("Ср", "ignore"),
		menu.Data("Чт", "ignore"), menu.Data("Пт", "ignore"), menu.Data("Сб", "ignore"), menu.Data("Вс", "ignore"),
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
	u, err = b.Stg.User().GetOrCreate(context.Background(), sender.ID, sender.Username, fmt.Sprintf("%s %s", sender.FirstName, sender.LastName))
	if err != nil {
		b.Log.Error("Failed to get or create user", logger.Error(err))
		return nil
	}
	return u
}
