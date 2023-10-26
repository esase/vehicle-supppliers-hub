package userservice

import "github.com/golang-jwt/jwt/v4"

type User struct {
	UserID        int
	AgencyID      int
	TopAgencyID   int
	ViaUserID     int
	CorrelationID string
	OriginalUAT   string
	Username      string
}

type parsedClaims struct {
	UserID        int    `json:"userId"`
	AgencyID      int    `json:"agencyId"`
	TopAgencyID   int    `json:"topAgencyId"`
	ViaUserID     int    `json:"viaUserId"`
	CorrelationID string `json:"correlationId"`
	jwt.RegisteredClaims
}
