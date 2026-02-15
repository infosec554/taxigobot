package bot

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"taxibot/pkg/logger"
	"taxibot/pkg/models"

	tele "gopkg.in/telebot.v3"
)

// Regex for Russian license plates: 1 letter, 3 digits, 2 letters, 2-3 digits
var licensePlateRegex = regexp.MustCompile(`^[АВЕКМНОРСТУХ]{1}[0-9]{3}[АВЕКМНОРСТУХ]{2}[0-9]{2,3}$`)

func normalizeLicensePlate(input string) string {
	mapping := map[rune]rune{
		'A': 'А', 'B': 'В', 'E': 'Е', 'K': 'К', 'M': 'М', 'H': 'Н', 'O': 'О', 'P': 'Р', 'C': 'С', 'T': 'Т', 'Y': 'У', 'X': 'Х',
	}
	var builder strings.Builder
	for _, r := range input {
		if cyr, ok := mapping[r]; ok {
			builder.WriteRune(cyr)
		} else {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func (b *Bot) handleDriverRegistrationStart(c tele.Context) error {
	// Step 1: Car Brand
	brands, err := b.Stg.Car().GetBrands(context.Background())
	if err != nil {
		b.Log.Error("Failed to get car brands", logger.Error(err))
		return c.Send("❌ Ошибка при загрузке списка автомобилей.")
	}

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	var currentRow []tele.Btn

	for i, brand := range brands {
		currentRow = append(currentRow, menu.Data(brand.Name, fmt.Sprintf("reg_brand_%d", brand.ID)))
		if (i+1)%3 == 0 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}

	// Add "Other" option if needed, but for now stick to DB
	menu.Inline(rows...)

	session := b.Sessions[c.Sender().ID]
	session.State = StateCarBrand

	return c.Send("🚗 <b>Выберите марку вашего автомобиля:</b>", menu, tele.ModeHTML)
}

func (b *Bot) handleCarBrandSelection(c tele.Context, brandID int64) error {
	user := b.getCurrentUser(c)
	session := b.Sessions[c.Sender().ID]
	if session.DriverProfile == nil {
		session.DriverProfile = &models.DriverProfile{UserID: user.ID}
	}

	// Fetch brand name for storage (simplification, storing name)
	// Ideally we store ID, but the task says "Save into driver_profiles table" which uses strings
	// based on my memory of migration. Let's check migration 0007 content again if unsure.
	// The migration has VARCHAR(255) for car_brand.
	// So fetching name is safer.
	// Actually, I can just get it from the button text? No, safer to fetch or use ID if possible.
	// But let's fetch models first.

	modelsList, err := b.Stg.Car().GetModels(context.Background(), brandID)
	if err != nil {
		return c.Send("❌ Ошибка при загрузке моделей.")
	}

	// Hack: I need the brand name.
	// I'll cheat and fetch all brands again or use a helper,
	// or assume the user selected from the list I just showed.
	// For now, let's fetch models.

	// Store Brand ID temporarily or Name?
	// Let's store Brand Name by fetching it relative to models? No.
	// Let's just fetch the brands list again filtered? No, `GetBrands` gets all.
	// Optimally I'd have GetBrandByID.
	// I'll skip storing the name *right now* and do it at the end or use a global cache?
	// Let's just update the profile with the Brand ID? No, table has VARCHAR.
	// Okay, I will add `GetBrand` to storage or just iterate.
	// Let's assume for now I can get it.
	// Actually, I can pass it in callback? No, data limit.

	// Let's iterate over brands to find name? Costly but safe.
	brands, _ := b.Stg.Car().GetBrands(context.Background())
	var brandName string
	for _, b := range brands {
		if b.ID == brandID {
			brandName = b.Name
			break
		}
	}
	session.DriverProfile.CarBrand = brandName

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	var currentRow []tele.Btn

	for i, m := range modelsList {
		currentRow = append(currentRow, menu.Data(m.Name, fmt.Sprintf("reg_model_%d", m.ID)))
		if (i+1)%3 == 0 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}

	// Add "Other"
	rows = append(rows, menu.Row(menu.Data("🖊 Другая", "reg_model_other")))

	menu.Inline(rows...)
	session.State = StateCarModel

	return c.Edit("🚗 <b>Выберите модель автомобиля:</b>", menu, tele.ModeHTML)
}

func (b *Bot) handleCarModelSelection(c tele.Context, modelID int64) error {
	session := b.Sessions[c.Sender().ID]

	// Need to find model name again
	// Current hack: I don't have GetModelByID easily accessible without brandID context usually.
	// But `driver_handlers.go` uses `GetModels` with brandID.
	// I stored brandName but not brandID in session.
	// I should probably store BrandID in session context or just TempString.
	// Let's assume I can't easily get it without another query.
	// I'll add `GetModel(id)` to storage?
	// Or just trust the button text if I could get it (not available in telebot callback context easily).

	// Simpler approach: Select * from car_models where id = modelID.
	// I haven't added `GetModelByID` to repo.
	// I will just ask for License Plate now.
	// Wait, I need to save the model name.
	// I will use `reg_model_<name>`? No, potentially long.
	// I will add `GetModelByID` to `car_repo.go` later?
	// Or I can use a raw query here for simplicity or add to repo.

	// Let's use a workaround: The callback data contains ID.
	// I'll implement `GetModelByID` in `car_repo`? No, I need to modify interface.
	// I'll just query standard QueryRow here? No, leakage.

	// Best way: Update `car_repo` to include `GetModelByID` or `GetBrandByID`.
	// For now, I will use `TempString` to store brandID?
	// Actually, let's just create a quick helper or modify the repo.
	// Or even simpler: usage of `GetModels` for the brand we selected?
	// But I only stored `CarBrand` string.

	// Let's use `TempString` to store BrandID in Step 1.

	// Re-think:
	// stored BrandName.
	// I need ModelName.
	// I can query all brands, find brand with that name, get its ID, get models, find model.
	// A bit convoluted but works without changing interface.

	// Optimized: Modify `car_repo` to have `GetModel(id)`.
	// I'll do that in a separate step if needed.
	// For now, let's assume I can get it.
	// I will execute a direct query via pool if allowed? No.

	// Let's use the convoluted way for safety:
	// Find brand ID by name (which I have).
	brands, _ := b.Stg.Car().GetBrands(context.Background())
	var brandID int64
	for _, br := range brands {
		if br.Name == session.DriverProfile.CarBrand {
			brandID = br.ID
			break
		}
	}

	modelsList, _ := b.Stg.Car().GetModels(context.Background(), brandID)
	for _, m := range modelsList {
		if m.ID == modelID {
			session.DriverProfile.CarModel = m.Name
			break
		}
	}

	session.State = StateLicensePlate
	return c.Edit("🔢 <b>Введите гос. номер автомобиля:</b>\n\nПример: <code>A123BC777</code> (русские буквы)", tele.ModeHTML)
}

func (b *Bot) handleCarModelOther(c tele.Context) error {
	session := b.Sessions[c.Sender().ID]
	session.State = StateCarModelOther
	return c.Edit("🖊 <b>Введите модель автомобиля вручную:</b>", tele.ModeHTML)
}

func (b *Bot) handleLicensePlateInput(c tele.Context) error {
	plate := strings.ToUpper(strings.TrimSpace(c.Text()))
	plate = normalizeLicensePlate(plate)

	// Validate
	if !licensePlateRegex.MatchString(plate) {
		return c.Send("❌ <b>Некорректный формат!</b>\n\nИспользуйте формат: <code>A123BC777</code> (Кириллица, 8-9 символов).", tele.ModeHTML)
	}

	session := b.Sessions[c.Sender().ID]
	session.DriverProfile.LicensePlate = plate

	// Save Profile
	if err := b.Stg.User().CreateDriverProfile(context.Background(), session.DriverProfile); err != nil {
		b.Log.Error("Failed to create driver profile", logger.Error(err))
		return c.Send("❌ Ошибка при сохранении данных.")
	}

	// Move to Routes
	session.State = StateIdle // Reset state for route handler

	c.Send("✅ Данные автомобиля сохранены!")

	// Start Route setup
	return b.handleAddRouteStart(c, session)
}

func (b *Bot) handleRegistrationCheck(c tele.Context) error {
	// Check if user has routes and tariffs
	user := b.getCurrentUser(c)
	routes, _ := b.Stg.Route().GetDriverRoutes(context.Background(), user.ID)
	enabledTariffs, _ := b.Stg.Tariff().GetEnabled(context.Background(), user.ID)

	if len(routes) == 0 {
		return c.Send("⚠️ <b>Необходимо добавить хотя бы один маршрут!</b>", tele.ModeHTML)
	}

	hasTariff := false
	for _, v := range enabledTariffs {
		if v {
			hasTariff = true
			break
		}
	}
	if !hasTariff {
		return c.Send("⚠️ <b>Необходимо выбрать хотя бы один тариф!</b>", tele.ModeHTML)
	}

	// Submit for review
	ctx := context.Background()
	b.Stg.User().UpdateStatusByID(ctx, user.ID, "pending_review")

	c.Send("🎉 <b>Регистрация завершена!</b>\n\nВаш профиль отправлен на проверку администратору. Ожидайте уведомления.", tele.ModeHTML)

	// Notify Admin
	// Build details string
	profile, _ := b.Stg.User().GetDriverProfile(context.Background(), user.ID)
	var carDetails string
	if profile != nil {
		carDetails = fmt.Sprintf("🚗 %s %s (%s)", profile.CarBrand, profile.CarModel, profile.LicensePlate)
	}

	tariffCount := 0
	for _, v := range enabledTariffs {
		if v {
			tariffCount++
		}
	}

	phone := "Не указан"
	if user.Phone != nil {
		phone = *user.Phone
	}

	msg := fmt.Sprintf("🔔 <b>НОВЫЙ ВОДИТЕЛЬ НА ПРОВЕРКЕ</b>\n\n👤 %s\n📞 %s\n%s\n\n📍 Маршрутов: %d\n🚕 Тарифov: %d",
		user.FullName, phone, carDetails, len(routes), tariffCount)

	// Send to admins — universal tugmalar (messages["ru"] dan)
	admins, _ := b.Stg.User().GetAll(context.Background())
	ru := messages["ru"]
	menu := &tele.ReplyMarkup{}
	menu.Inline(menu.Row(
		menu.Data(ru["admin_btn_approve"], fmt.Sprintf("approve_driver_%d", user.ID)),
		menu.Data(ru["admin_btn_reject"], fmt.Sprintf("reject_driver_%d", user.ID)),
	))

	sentCount := 0
	for _, u := range admins {
		if u.Role == "admin" {
			b.Log.Info("Notifying admin about new driver", logger.Int64("admin_id", u.TelegramID), logger.Int64("driver_id", user.ID))
			_, err := b.Bot.Send(&tele.User{ID: u.TelegramID}, msg, menu, tele.ModeHTML)
			if err != nil {
				b.Log.Error("Failed to notify admin", logger.Error(err), logger.Int64("admin_id", u.TelegramID))
			} else {
				sentCount++
			}
		}
	}
	b.Log.Info("Driver registration notifications sent", logger.Int("sent_count", sentCount))

	return nil
}
