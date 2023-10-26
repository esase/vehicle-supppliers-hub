package json

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type LocationsRS struct {
	Errors
	Stations []LocationsRSStation `json:"stations"`
	Meta     PaginationMeta       `json:"meta"`
}

type LocationsRSStation struct {
	Id         int                          `json:"id"`
	Code       string                       `json:"code"`
	Name       string                       `json:"name"`
	Country    LocationsRSStationCountry    `json:"country"`
	City       string                       `json:"city"`
	Address    string                       `json:"address"`
	ZipCode    string                       `json:"zip_code"`
	Latitude   string                       `json:"latitude"`
	Longitude  string                       `json:"longitude"`
	Phone      string                       `json:"phone"`
	OutOfHours LocationsRSStationOutOfHours `json:"out_of_hours"`
	Schedule   LocationsRSStationSchedule   `json:"schedule"`
}

func (s *LocationsRSStation) GetLatitude() *float32 {
	var latitude float32

	convertedLatitude, err := strconv.ParseFloat(s.Latitude, 32)
	if err == nil {
		latitude = float32(convertedLatitude)
	}

	return &latitude
}

func (s *LocationsRSStation) GetLongitude() *float32 {
	var longitude float32

	convertedLongitude, err := strconv.ParseFloat(s.Longitude, 32)
	if err == nil {
		longitude = float32(convertedLongitude)
	}

	return &longitude
}

func (s *LocationsRSStation) GetRawData() struct {
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
} {
	rawData := ""

	jsonData, err := json.Marshal(s)
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

type LocationsRSStationCountry struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type LocationsRSStationOutOfHours struct {
	ChargeOnPickup  bool `json:"charge_on_pickup"`
	ChargeOnDropOff bool `json:"charge_on_dropoff"`
}

type LocationsRSStationSchedule struct {
	Week     LocationsRSStationScheduleInfo `json:"week"`
	Saturday LocationsRSStationScheduleInfo `json:"saturday"`
	Sunday   LocationsRSStationScheduleInfo `json:"sunday"`
	Holiday  LocationsRSStationScheduleInfo `json:"holiday"`
}

type LocationsRSStationScheduleInfo struct {
	OpenTime            string `json:"open_time"`
	CloseTime           string `json:"close_time"`
	LunchStartTime      string `json:"lunch_start_time"`
	LunchEndTime        string `json:"lunch_end_time"`
	OutOfHoursStartTime string `json:"outofhours_start_time"`
	OutOfHoursEndTime   string `json:"outofhours_end_time"`
}

func (i *LocationsRSStationScheduleInfo) GetOpenTime() string {
	openTime := strings.Split(i.OpenTime, ":")

	return fmt.Sprintf("%v:%v", openTime[0], openTime[1])
}

func (i *LocationsRSStationScheduleInfo) GetCloseTime() string {
	closeTime := strings.Split(i.CloseTime, ":")

	return fmt.Sprintf("%v:%v", closeTime[0], closeTime[1])
}

func (i *LocationsRSStationScheduleInfo) GetOohOpenTime() string {
	openTime := strings.Split(i.OutOfHoursStartTime, ":")

	return fmt.Sprintf("%v:%v", openTime[0], openTime[1])
}

func (i *LocationsRSStationScheduleInfo) GetOohCloseTime() string {
	closeTime := strings.Split(i.OutOfHoursEndTime, ":")

	return fmt.Sprintf("%v:%v", closeTime[0], closeTime[1])
}
