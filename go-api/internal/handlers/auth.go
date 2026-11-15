package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Simplepavel/request-monitoring/go-api/internal/service"
)

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

type registerRequest struct {
	LastName   string `json:"last_name"`
	FirstName  string `json:"first_name"`
	MiddleName string `json:"middle_name"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	Password   string `json:"password"`
}

func (req *registerRequest) normalize() {
	req.LastName = strings.TrimSpace(req.LastName)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.MiddleName = strings.TrimSpace(req.MiddleName)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
}

func (req registerRequest) validate() error {
	if err := validateName(req.LastName, "Фамилия", true); err != nil {
		return err
	}
	if err := validateName(req.FirstName, "Имя", true); err != nil {
		return err
	}
	if err := validateName(req.MiddleName, "Отчество", false); err != nil {
		return err
	}

	if !phonePattern.MatchString(req.Phone) {
		return errors.New("телефон укажите в международном формате, например +79991234567")
	}

	if utf8.RuneCountInString(req.Email) > 255 {
		return errors.New("электронная почта слишком длинная")
	}

	parsedEmail, err := mail.ParseAddress(req.Email)
	if err != nil || parsedEmail.Address != req.Email {
		return errors.New("укажите корректную электронную почту")
	}

	if utf8.RuneCountInString(req.Password) < 12 {
		return errors.New("пароль должен содержать не менее 12 символов")
	}
	if len(req.Password) > 72 {
		return errors.New("пароль слишком длинный")
	}

	return nil
}

func validateName(value, field string, required bool) error {
	if value == "" && required {
		return errors.New(field + " обязательно")
	}

	if strings.ContainsAny(value, "\r\n\t") {
		return errors.New(field + " не должно содержать переносы строк или табуляцию")
	}

	if utf8.RuneCountInString(value) > 100 {
		return errors.New(field + " слишком длинное")
	}

	return nil
}

type registerResponse struct {
	ID   int64  `json:"id"`
	Role string `json:"role"`
}

func (h *Handler) RegisterResident(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	var req registerRequest
	if err := decoder.Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Некорректный JSON")
		return
	}

	req.normalize()

	if err := req.validate(); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	userID, err := h.authService.RegisterResident(ctx, service.RegisterResidentInput{
		LastName:   req.LastName,
		FirstName:  req.FirstName,
		MiddleName: req.MiddleName,
		Phone:      req.Phone,
		Email:      req.Email,
		Password:   req.Password,
	})
	if err != nil {
		log.Printf("Ошибка регистрации жителя: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Не удалось зарегистрироваться")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	err = json.NewEncoder(w).Encode(registerResponse{
		ID:   userID,
		Role: "resident",
	})
	if err != nil {
		log.Printf("Ошибка отправки ответа: %v", err)
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
