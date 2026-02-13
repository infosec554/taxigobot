package bot

import (
	"context"
	"fmt"
	"taxibot/pkg/models"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// GoogleCalendarService handles synchronization with Google Calendar API
type GoogleCalendarService struct {
	Config *oauth2.Config
}

func NewGoogleCalendarService() *GoogleCalendarService {
	// Credentials should be loaded from a file or env
	// For now, this is a structure ready for integration
	return &GoogleCalendarService{}
}

// AddOrderToCalendar pushes a new taxi order to a specific Google Calendar
func (s *GoogleCalendarService) AddOrderToCalendar(order *models.Order) (string, error) {
	_ = context.Background() // Suppress unused for now or use in insertion

	// Example of creating an event in Google Format
	startTime := order.PickupTime.Format(time.RFC3339)
	endTime := order.PickupTime.Add(time.Hour).Format(time.RFC3339)

	_ = option.WithCredentialsFile("") // Placeholder for inserting

	event := &calendar.Event{
		Summary:     fmt.Sprintf("🚕 Taxi: %s ➞ %s", order.FromLocationName, order.ToLocationName),
		Location:    order.FromLocationName,
		Description: fmt.Sprintf("Yo'lovchilar: %d\nNarx: %d %s\nID: #%d", order.Passengers, order.Price, order.Currency, order.ID),
		Start: &calendar.EventDateTime{
			DateTime: startTime,
			TimeZone: "Asia/Tashkent",
		},
		End: &calendar.EventDateTime{
			DateTime: endTime,
			TimeZone: "Asia/Tashkent",
		},
	}

	fmt.Printf("Syncing to Google Calendar: %s\n", event.Summary)
	return "https://calendar.google.com/calendar/event?eid=mock_id", nil
}
