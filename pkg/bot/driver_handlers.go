package bot

import (
	"context"
	"fmt"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"
	"time"

	tele "gopkg.in/telebot.v3"
)

func (b *Bot) handleDriverTariffs(c tele.Context) error {
	return b.showDriverTariffs(c, false)
}

func (b *Bot) showDriverTariffs(c tele.Context, deleteMode bool) error {
	user := b.getCurrentUser(c)
	tariffs, _ := b.Stg.Tariff().GetAll(context.Background())
	enabled, _ := b.Stg.Tariff().GetEnabled(context.Background(), user.ID)

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	var currentRow []tele.Btn
	for i, t := range tariffs {
		var text, data string
		if deleteMode {
			text = fmt.Sprintf("🗑 %s", t.Name)
			data = fmt.Sprintf("del_tf_%d", t.ID)
		} else {
			icon := "🔴"
			if enabled[t.ID] {
				icon = "✅"
			}
			text = fmt.Sprintf("%s %s", icon, t.Name)
			data = fmt.Sprintf("tgl_%d", t.ID)
		}

		currentRow = append(currentRow, menu.Data(text, data))
		if (i+1)%2 == 0 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}

	// Control buttons
	var controls []tele.Btn
	if deleteMode {
		controls = append(controls, menu.Data("🔙 Назад", "tf_back"))
	} else {
		controls = append(controls, menu.Data("🗑 Удалить", "tf_del_mode"))
		controls = append(controls, menu.Data("✅ Готово", "tf_done"))
	}
	rows = append(rows, menu.Row(controls...))

	menu.Inline(rows...)

	msg := "<b>🚕 Мои тарифы</b>\n\nИз каких тарифов вы хотите получать заказы? Выберите:"
	if deleteMode {
		msg = "<b>🗑 Режим удаления</b>\n\nВыберите тариф, который хотите удалить:"
	}

	if c.Callback() != nil {
		return c.Edit(msg, menu, tele.ModeHTML)
	}
	return c.Send(msg, menu, tele.ModeHTML)
}

func (b *Bot) handleDriverRoutes(c tele.Context) error {
	user := b.getCurrentUser(c)
	routes, _ := b.Stg.Route().GetDriverRoutes(context.Background(), user.ID)

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	txt := "<b>📍 Мои маршруты</b>\n\n"
	if len(routes) == 0 {
		txt += "Вы еще не добавили ни одного маршрута."
	} else {
		for i, r := range routes {
			from, _ := b.Stg.Location().GetByID(context.Background(), r[0])
			to, _ := b.Stg.Location().GetByID(context.Background(), r[1])
			fromName, toName := "Неизвестно", "Неизвестно"
			if from != nil {
				fromName = from.Name
			}
			if to != nil {
				toName = to.Name
			}
			txt += fmt.Sprintf("%d. %s ➡️ %s\n", i+1, fromName, toName)
		}
	}

	rows = append(rows, menu.Row(menu.Data("➕ Добавить новый", "add_route")))
	if len(routes) > 0 {
		rows = append(rows, menu.Row(menu.Data("🗑 Очистить", "clear_routes")))
		rows = append(rows, menu.Row(menu.Data("✅ Далее", "routes_done")))
	}

	menu.Inline(rows...)
	if c.Callback() != nil {
		return c.Edit(txt, menu, tele.ModeHTML)
	}
	return c.Send(txt, menu, tele.ModeHTML)
}

func (b *Bot) handleAddRouteStart(c tele.Context, session *UserSession) error {
	session.State = StateDriverRouteFrom
	locations, _ := b.Stg.Location().GetAll(context.Background())

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	var currentRow []tele.Btn
	for i, l := range locations {
		currentRow = append(currentRow, menu.Data(l.Name, fmt.Sprintf("dr_f_%d", l.ID)))
		if (i+1)%3 == 0 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}
	menu.Inline(rows...)

	msg := "<b>📍 Откуда вы выезжаете?</b>\nВыберите город:"
	if c.Callback() != nil {
		return c.Edit(msg, menu, tele.ModeHTML)
	}
	return c.Send(msg, menu, tele.ModeHTML)
}

func (b *Bot) handleDriverCalendarSearch(c tele.Context) error {
	now := time.Now()
	return b.generateCalendarWithPrefix(c, now.Year(), int(now.Month()), "sc_cal_")
}

func (b *Bot) handleDriverAgenda(c tele.Context) error {
	user := b.getCurrentUser(c)
	if user.Status != "active" {
		return c.Send("🚫 <b>Доступ запрещен!</b>\n\nВаш профиль находится на проверке или заблокирован.", tele.ModeHTML)
	}

	orders, _ := b.Stg.Order().GetActiveOrders(context.Background())
	if len(orders) == 0 {
		return c.Send("Активных заказов пока нет.")
	}

	// Group by date
	groups := make(map[string][]models.Order)
	var dates []string
	for _, o := range orders {
		if o.PickupTime == nil {
			continue
		}
		d := o.PickupTime.Format("2006-01-02")
		if _, ok := groups[d]; !ok {
			dates = append(dates, d)
		}
		groups[d] = append(groups[d], *o)
	}

	txt := "<b>📋 Список заказов на неделю:</b>\n\n"
	for _, d := range dates {
		parsedDate, _ := time.Parse("2006-01-02", d)
		txt += fmt.Sprintf("📅 <b>%s</b>\n", parsedDate.Format("02.01.2006"))
		for _, o := range groups[d] {
			txt += fmt.Sprintf("▫️ %s: <b>%s ➞ %s</b> (#%d)\n",
				o.PickupTime.Format("15:04"), o.FromLocationName, o.ToLocationName, o.ID)
		}
		txt += "\n"
	}

	txt += "<i>Для подробной информации введите ID заказа.</i>"
	return c.Send(txt, tele.ModeHTML)
}

func (b *Bot) handleDriverDateSearch(c tele.Context, dateStr string) error {
	return b.driverDateSearchLogic(c, dateStr, false)
}

func (b *Bot) handleDriverDateSearchAll(c tele.Context, dateStr string) error {
	return b.driverDateSearchLogic(c, dateStr, true)
}

func (b *Bot) driverDateSearchLogic(c tele.Context, dateStr string, showAll bool) error {
	// showAll is now ignored as we always show all
	layout := "2006-01-02"
	date, err := time.Parse(layout, dateStr)
	if err != nil {
		return c.Send("❌ Неверный формат даты.")
	}

	// Get current user ID (driver ID)
	user := b.getCurrentUser(c)
	if user.Status != "active" {
		return c.Send("🚫 <b>Доступ запрещен!</b>\n\nВаш профиль находится на проверке или заблокирован.", tele.ModeHTML)
	}

	orders, err := b.Stg.Order().GetOrdersByDate(context.Background(), date, user.ID)
	if err != nil {
		return c.Send("❌ Произошла ошибка.")
	}

	if len(orders) == 0 {
		return c.Send(fmt.Sprintf("📅 На <b>%s</b> активных заказов не найдено.", date.Format("02.01.2006")), tele.ModeHTML)
	}

	c.Send(fmt.Sprintf("📅 <b>Заказы на %s:</b>", date.Format("02.01.2006")), tele.ModeHTML)

	loc := time.FixedZone("Europe/Moscow", 3*60*60)
	for _, o := range orders {
		timeStr := "Неизвестно"
		if o.PickupTime != nil {
			timeStr = o.PickupTime.In(loc).Format("15:04")
		}

		txt := fmt.Sprintf("📦 <b>Заказ #%d</b>\n📍 %s ➡️ %s\n👥 %d чел.\n🕒 %s\n💰 %d %s",
			o.ID, o.FromLocationName, o.ToLocationName, o.Passengers, timeStr, o.Price, o.Currency)

		btnMenu := &tele.ReplyMarkup{}
		if o.Status == "active" {
			btnMenu.Inline(btnMenu.Row(
				btnMenu.Data("📥 Принять заказ", fmt.Sprintf("take_%d", o.ID)),
			))
		}
		c.Send(txt, btnMenu, tele.ModeHTML)
	}

	return nil
}

func (b *Bot) handleAddRouteTo(c tele.Context, session *UserSession) error {
	session.State = StateDriverRouteTo
	locations, _ := b.Stg.Location().GetAll(context.Background())

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	var currentRow []tele.Btn
	for _, l := range locations {
		if l.ID != session.OrderData.FromLocationID { // Reuse OrderData for temp storage of Route From
			currentRow = append(currentRow, menu.Data(l.Name, fmt.Sprintf("dr_t_%d", l.ID)))
			if (len(currentRow))%3 == 0 {
				rows = append(rows, menu.Row(currentRow...))
				currentRow = []tele.Btn{}
			}
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}
	menu.Inline(rows...)

	return c.Edit("<b>🏁 Куда вы едете?</b>\nВыберите город:", menu, tele.ModeHTML)
}

func (b *Bot) handleAddRouteComplete(c tele.Context, session *UserSession) error {
	user := b.getCurrentUser(c)
	if user == nil {
		return c.Send("❌ Ошибка: Пользователь не найден. Пожалуйста, нажмите /start.")
	}

	b.Log.Info("Adding Route", logger.Int64("driver_id", user.ID), logger.Int64("from", session.OrderData.FromLocationID), logger.Int64("to", session.OrderData.ToLocationID))

	if session.OrderData.FromLocationID == 0 || session.OrderData.ToLocationID == 0 {
		return c.Send("❌ Ошибка: Некорректные данные маршрута. Попробуйте еще раз.")
	}

	err := b.Stg.Route().AddRoute(context.Background(), user.ID, session.OrderData.FromLocationID, session.OrderData.ToLocationID)
	if err != nil {
		b.Log.Error("Failed to add route", logger.Error(err))
		return c.Send("❌ Ошибка при сохранении маршрута.")
	}

	c.Send("✅ Маршрут добавлен!")

	// Har doim marshrut ro'yxatini ko'rsat (registration yoki oddiy rejim)
	// handleRegistrationCheck faqat tf_done orqali chaqiriladi
	return b.handleDriverRoutes(c)
}
