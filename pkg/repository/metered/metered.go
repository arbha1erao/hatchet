package metered

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/hatchet-dev/hatchet/pkg/repository"
	"github.com/hatchet-dev/hatchet/pkg/repository/cache"
	"github.com/hatchet-dev/hatchet/pkg/repository/postgres/dbsqlc"
)

type Metered struct {
	entitlements repository.EntitlementsRepository
	l            *zerolog.Logger
	c            cache.Cacheable
}

func (m *Metered) Stop() {
	m.c.Stop()
}

func NewMetered(entitlements repository.EntitlementsRepository, l *zerolog.Logger) *Metered {
	return &Metered{
		entitlements: entitlements,
		l:            l,
		c:            cache.New(time.Second * 30),
	}
}

func (m *Metered) canCreate(ctx context.Context, resource dbsqlc.LimitResource, tenantId string, numberOfResources int32) (bool, int, error) {
	var key = fmt.Sprintf("%s:%s", resource, tenantId)

	var canCreate *bool
	var percent int

	if hit, ok := m.c.Get(key); ok {
		c := hit.(bool)
		canCreate = &c
	}

	if canCreate == nil {
		c, p, err := m.entitlements.TenantLimit().CanCreate(ctx, resource, tenantId, numberOfResources)

		if err != nil {
			return false, 0, fmt.Errorf("could not check tenant limit: %w", err)
		}

		canCreate = &c
		percent = p

		if percent <= 50 || percent >= 100 {
			m.c.Set(key, c)
		}
	}

	return *canCreate, percent, nil
}

func (m *Metered) Meter(ctx context.Context, resource dbsqlc.LimitResource, tenantId string, numberOfResources int32) (precommit func() error, postcommit func()) {
	return func() error {
			canCreate, _, err := m.canCreate(ctx, resource, tenantId, numberOfResources)

			if err != nil {
				return fmt.Errorf("could not check tenant limit: %w", err)
			}

			if !canCreate {
				return ErrResourceExhausted
			}

			return nil
		}, func() {
			_, percent, err := m.canCreate(ctx, resource, tenantId, numberOfResources)

			if err != nil {
				m.l.Error().Err(err).Msg("could not check tenant limit")
				return
			}

			limit, err := m.entitlements.TenantLimit().Meter(ctx, resource, tenantId, numberOfResources)

			if limit != nil && (percent <= 50 || percent >= 100) {
				var key = fmt.Sprintf("%s:%s", resource, tenantId)
				m.c.Set(key, limit.Value < limit.LimitValue)
			}

			if err != nil {
				m.l.Error().Err(err).Msg("could not meter resource")
			}
		}
}

var ErrResourceExhausted = fmt.Errorf("resource exhausted")

func MakeMetered[T any](ctx context.Context, m *Metered, resource dbsqlc.LimitResource, tenantId string, numberOfResources int32, f func() (*string, *T, error)) (*T, error) {

	var key = fmt.Sprintf("%s:%s", resource, tenantId)

	var canCreate *bool
	var percent int

	if hit, ok := m.c.Get(key); ok {
		c := hit.(bool)
		canCreate = &c
	}

	if canCreate == nil {
		c, percent, err := m.entitlements.TenantLimit().CanCreate(ctx, resource, tenantId, numberOfResources)

		if err != nil {
			return nil, fmt.Errorf("could not check tenant limit: %w", err)
		}

		canCreate = &c

		if percent <= 50 || percent >= 100 {
			m.c.Set(key, c)
		}

	}

	if !*canCreate {
		return nil, ErrResourceExhausted
	}

	_, res, err := f()

	if err != nil {
		return nil, err
	}

	deferredMeter := func() {
		limit, err := m.entitlements.TenantLimit().Meter(ctx, resource, tenantId, numberOfResources)

		if limit != nil && (percent <= 50 || percent >= 100) {
			m.c.Set(key, limit.Value < limit.LimitValue)
		}

		// TODO: we should probably publish an event here if limits are exhausted to notify immediately

		if err != nil {
			m.l.Error().Err(err).Msg("could not meter resource")
		}
	}

	defer deferredMeter()

	return res, nil
}
