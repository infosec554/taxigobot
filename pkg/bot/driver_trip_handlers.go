package bot

import (
	"context"

	tele "gopkg.in/telebot.v3"
)

func (b *Bot) handleDriverOnWay(c tele.Context, orderID int64) error {
	err := b.Stg.Order().SetOrderOnWay(context.Background(), orderID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "❌ Ошибка (Возможно, статус изменился)"})
	}

	b.notifyUser(b.getOrderClientID(orderID), "🚖 Водитель выехал к вам!")
	c.Respond(&tele.CallbackResponse{Text: "Статус: Выехал"})
	return b.handleMyOrdersDriver(c)
}

func (b *Bot) handleDriverArrived(c tele.Context, orderID int64) error {
	err := b.Stg.Order().SetOrderArrived(context.Background(), orderID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "❌ Ошибка"})
	}

	b.notifyUser(b.getOrderClientID(orderID), "🚖 Водитель прибыл на место!")
	c.Respond(&tele.CallbackResponse{Text: "Статус: Прибыл"})
	return b.handleMyOrdersDriver(c)
}

func (b *Bot) handleDriverStartTrip(c tele.Context, orderID int64) error {
	err := b.Stg.Order().SetOrderInProgress(context.Background(), orderID)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "❌ Ошибка"})
	}

	b.notifyUser(b.getOrderClientID(orderID), "▶ Поездка началась!")
	c.Respond(&tele.CallbackResponse{Text: "Статус: В пути"})
	return b.handleMyOrdersDriver(c)
}

// Helper to get client ID for notification
func (b *Bot) getOrderClientID(orderID int64) int64 {
	order, _ := b.Stg.Order().GetByID(context.Background(), orderID)
	if order != nil {
		return order.ClientID
	}
	return 0
}
