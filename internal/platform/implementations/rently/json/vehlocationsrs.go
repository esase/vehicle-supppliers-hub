package json

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"bitbucket.org/crgw/supplier-hub/internal/schema"
)

type PlaceRS struct {
	Id                int                      `json:"id"`
	Country           string                   `json:"country"`
	City              string                   `json:"city"`
	Phone             string                   `json:"phone"`
	Email             string                   `json:"email"`
	SupplierId        int                      `json:"supplierId"`
	Address           *string                  `json:"address,omitempty"`
	Address2          *string                  `json:"address2,omitempty"`
	Longitude         float32                  `json:"longitude"`
	Latitude          float32                  `json:"latitude"`
	Type              string                   `json:"type"`
	ServiceType       string                   `json:"serviceType"`
	Iata              *string                  `json:"iata,omitempty"`
	AttentionSchedule PlaceRSAttentionSchedule `json:"attentionSchedule"`
}

func (p *PlaceRS) GetRawData() struct {
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
} {
	rawData := ""

	jsonData, err := json.Marshal(p)
	if err == nil {
		rawData = string(jsonData)
	}

	return struct {
		Content     string `json:"content"`
		ContentType string `json:"contentType"`
	}{
		Content:     rawData,
		ContentType: "application/json",
	}
}

type PlaceRSAttentionSchedule struct {
	Id                int                          `json:"id"`
	TimezoneId        string                       `json:"timezoneId"`
	TimezoneUTCOffset string                       `json:"timezoneUTCOffset"`
	GapForBookingTime int                          `json:"gapForBookingTime"`
	Gap               int                          `json:"gap"`
	Schedule          PlaceRSAttentionScheduleInfo `json:"schedule"`
}

type PlaceRSAttentionScheduleInfo struct {
	Monday    []PlaceRSAttentionScheduleInfoTime `json:"monday"`
	Tuesday   []PlaceRSAttentionScheduleInfoTime `json:"tuesday"`
	Wednesday []PlaceRSAttentionScheduleInfoTime `json:"wednesday"`
	Thursday  []PlaceRSAttentionScheduleInfoTime `json:"thursday"`
	Friday    []PlaceRSAttentionScheduleInfoTime `json:"friday"`
	Saturday  []PlaceRSAttentionScheduleInfoTime `json:"saturday"`
	Sunday    []PlaceRSAttentionScheduleInfoTime `json:"sunday"`
	Holiday   []PlaceRSAttentionScheduleInfoTime `json:"holiday"`
}

func (i *PlaceRSAttentionScheduleInfo) GetOpeningTime() []schema.OpeningTime {
	weekDays := map[int][]PlaceRSAttentionScheduleInfoTime{
		1: i.Monday,
		2: i.Tuesday,
		3: i.Wednesday,
		4: i.Thursday,
		5: i.Friday,
		6: i.Saturday,
		7: i.Sunday,
	}
	weekDaysKeys := make([]int, 0)
	for k := range weekDays {
		weekDaysKeys = append(weekDaysKeys, k)
	}
	sort.Ints(weekDaysKeys)

	openingTime := []schema.OpeningTime{}

	for _, key := range weekDaysKeys {
		for _, schedule := range weekDays[key] {
			openingTime = append(openingTime, schema.OpeningTime{
				Open:    true,
				Weekday: key,
				Start:   schedule.GetOpenedFrom(),
				End:     schedule.GetOpenedTo(),
			})
		}
	}

	return openingTime
}

type PlaceRSAttentionScheduleInfoTime struct {
	OpenedFrom string `json:"openedFrom"`
	OpenedTo   string `json:"openedTo"`
}

func (t *PlaceRSAttentionScheduleInfoTime) GetOpenedFrom() string {
	openedFromTime := strings.Split(t.OpenedFrom, ":")

	return fmt.Sprintf("%v:%v", openedFromTime[0], openedFromTime[1])
}

func (t *PlaceRSAttentionScheduleInfoTime) GetOpenedTo() string {
	openedToTime := strings.Split(t.OpenedTo, ":")

	return fmt.Sprintf("%v:%v", openedToTime[0], openedToTime[1])
}
