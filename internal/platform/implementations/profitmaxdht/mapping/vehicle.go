package mapping

import "bitbucket.org/crgw/supplier-hub/internal/schema"

func DistanceUnit(value string) *schema.MileageDistanceUnit {
	var mapped schema.MileageDistanceUnit

	switch value {
	case string(schema.Mile):
		mapped = schema.Mile

	case string(schema.Km):
		mapped = schema.Km
	}

	if mapped == "" {
		return nil
	}

	return &mapped
}

func PeriodUnit(value string) *schema.MileagePeriodUnit {
	var mapped schema.MileagePeriodUnit

	switch value {
	case string(schema.Day):
		mapped = schema.Day

	case string(schema.Hour):
		mapped = schema.Hour

	case string(schema.RentalPeriod):
		mapped = schema.RentalPeriod
	}

	if mapped == "" {
		return nil
	}

	return &mapped
}

func FuelType(value string) *schema.VehicleFuelType {
	var mapped schema.VehicleFuelType

	switch value {
	case string(schema.Diesel):
		mapped = schema.Diesel

	case string(schema.Electric):
		mapped = schema.Electric

	case string(schema.Ethanol):
		mapped = schema.Ethanol

	case string(schema.Gas):
		mapped = schema.Gas

	case string(schema.Hybrid):
		mapped = schema.Hybrid

	case string(schema.Hydrogen):
		mapped = schema.Hydrogen

	case string(schema.MultiFuel):
		mapped = schema.MultiFuel
	case string(schema.Petrol):
		mapped = schema.Petrol
	}

	if mapped == "" {
		return nil
	}

	return &mapped
}

func Transmission(value string) *schema.VehicleTransmissionType {
	var mapped schema.VehicleTransmissionType

	switch value {
	case string(schema.Automatic):
		mapped = schema.Automatic
	case string(schema.Manual):
		mapped = schema.Manual
	}

	if mapped == "" {
		return nil
	}

	return &mapped
}

func DriveType(value string) *schema.VehicleDriveType {
	var mapped schema.VehicleDriveType

	switch value {
	case string(schema.AWD):
		mapped = schema.AWD

	case string(schema.N4WD):
		mapped = schema.N4WD
	}

	if mapped == "" {
		return nil
	}

	return &mapped
}
