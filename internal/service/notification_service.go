package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/wujunqi/rc_wujunqi/internal/config"
	"github.com/wujunqi/rc_wujunqi/internal/model"
)

type CreateNotificationInput struct {
	AppID          string
	Vendor         string
	TargetURL      string
	Method         string
	Headers        map[string]string
	Payload        json.RawMessage
	IdempotencyKey string
	MaxRetries     int
}

type NotificationRepository interface {
	CreateTaskWithOutbox(ctx context.Context, input CreateNotificationInput) (*model.NotificationTask, bool, error)
	GetTaskForApp(ctx context.Context, appID, id string) (*model.NotificationTask, error)
}

type NotificationService struct {
	repo           NotificationRepository
	vendorProfiles map[string]config.VendorProfile
	appCredentials map[string]config.AppCredential
}

func NewNotificationService(repo NotificationRepository) *NotificationService {
	return NewNotificationServiceWithConfig(repo, config.DefaultVendorProfiles(), config.DefaultAppCredentials())
}

func NewNotificationServiceWithConfig(repo NotificationRepository, vendorProfiles map[string]config.VendorProfile, appCredentials map[string]config.AppCredential) *NotificationService {
	return &NotificationService{
		repo:           repo,
		vendorProfiles: cloneVendorProfiles(vendorProfiles),
		appCredentials: cloneAppCredentials(appCredentials),
	}
}

func (s *NotificationService) Create(ctx context.Context, input CreateNotificationInput) (*model.NotificationTask, bool, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return nil, false, err
	}
	enriched, err := s.applyConfiguredProfile(normalized)
	if err != nil {
		return nil, false, err
	}
	task, created, err := s.repo.CreateTaskWithOutbox(ctx, enriched)
	if err != nil {
		return nil, false, err
	}
	if !created && !sameRequest(task, enriched) {
		return nil, false, ErrIdempotencyConflict
	}
	return task, created, nil
}

func (s *NotificationService) Get(ctx context.Context, appID, id string) (*model.NotificationTask, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, ErrInvalidNotification
	}
	task, err := s.repo.GetTaskForApp(ctx, appID, id)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func normalizeInput(input CreateNotificationInput) (CreateNotificationInput, error) {
	input.AppID = strings.TrimSpace(input.AppID)
	input.Vendor = strings.TrimSpace(input.Vendor)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.AppID == "" || input.Vendor == "" || len(input.Payload) == 0 {
		return input, ErrInvalidNotification
	}
	compacted := bytes.Buffer{}
	if err := json.Compact(&compacted, input.Payload); err != nil {
		return input, errors.Join(ErrInvalidNotification, err)
	}
	input.Payload = compacted.Bytes()
	return input, nil
}

func (s *NotificationService) applyConfiguredProfile(input CreateNotificationInput) (CreateNotificationInput, error) {
	credential, ok := s.appCredentials[input.AppID]
	if !ok || !credential.Enabled {
		return input, ErrForbidden
	}
	if !slices.Contains(credential.AllowedVendors, input.Vendor) {
		return input, ErrForbidden
	}

	profile, ok := s.vendorProfiles[input.Vendor]
	if !ok {
		return input, ErrInvalidNotification
	}

	input.TargetURL = strings.TrimSpace(profile.TargetURL)
	input.Method = strings.ToUpper(strings.TrimSpace(profile.Method))
	if input.Method == "" {
		input.Method = http.MethodPost
	}
	input.Headers = maps.Clone(profile.DefaultHeaders)
	if input.Headers == nil {
		input.Headers = map[string]string{}
	}
	if input.MaxRetries <= 0 {
		input.MaxRetries = profile.MaxRetries
	}
	if input.MaxRetries <= 0 {
		input.MaxRetries = 5
	}

	if input.TargetURL == "" {
		return input, ErrInvalidNotification
	}
	if _, err := url.ParseRequestURI(input.TargetURL); err != nil {
		return input, errors.Join(ErrInvalidNotification, err)
	}
	switch input.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
	default:
		return input, errors.Join(ErrInvalidNotification, errors.New("method must be POST, PUT, or PATCH"))
	}
	return input, nil
}

func sameRequest(task *model.NotificationTask, input CreateNotificationInput) bool {
	headersJSON, _ := json.Marshal(input.Headers)
	return task.AppID == input.AppID &&
		task.Vendor == input.Vendor &&
		task.TargetURL == input.TargetURL &&
		task.Method == input.Method &&
		bytes.Equal(task.HeadersJSON, headersJSON) &&
		bytes.Equal(task.PayloadJSON, input.Payload)
}

func cloneVendorProfiles(profiles map[string]config.VendorProfile) map[string]config.VendorProfile {
	cloned := make(map[string]config.VendorProfile, len(profiles))
	for vendor, profile := range profiles {
		profile.DefaultHeaders = maps.Clone(profile.DefaultHeaders)
		cloned[vendor] = profile
	}
	return cloned
}

func cloneAppCredentials(credentials map[string]config.AppCredential) map[string]config.AppCredential {
	cloned := make(map[string]config.AppCredential, len(credentials))
	for appID, credential := range credentials {
		credential.AllowedVendors = slices.Clone(credential.AllowedVendors)
		cloned[appID] = credential
	}
	return cloned
}
