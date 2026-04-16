//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// resetQuotaUserSubRepoStub 支持 GetByID、ResetDailyUsage、ResetWeeklyUsage、ResetMonthlyUsage，
// 其余方法继承 userSubRepoNoop（panic）。
type resetQuotaUserSubRepoStub struct {
	userSubRepoNoop

	sub *UserSubscription

	extendExpiryCalled bool
	updateStatusCalled bool
	extendedTo         time.Time
	updatedStatusTo    string
	extendExpiryErr    error
	updateStatusErr    error

	resetDailyCalled   bool
	resetWeeklyCalled  bool
	resetMonthlyCalled bool
	resetDailyErr      error
	resetWeeklyErr     error
	resetMonthlyErr    error
}

func (r *resetQuotaUserSubRepoStub) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	if r.sub == nil || r.sub.ID != id {
		return nil, ErrSubscriptionNotFound
	}
	cp := *r.sub
	return &cp, nil
}

func (r *resetQuotaUserSubRepoStub) ResetDailyUsage(_ context.Context, _ int64, windowStart time.Time) error {
	r.resetDailyCalled = true
	if r.resetDailyErr == nil && r.sub != nil {
		r.sub.DailyUsageUSD = 0
		r.sub.DailyWindowStart = &windowStart
	}
	return r.resetDailyErr
}

func (r *resetQuotaUserSubRepoStub) ResetWeeklyUsage(_ context.Context, _ int64, windowStart time.Time) error {
	r.resetWeeklyCalled = true
	if r.resetWeeklyErr == nil && r.sub != nil {
		r.sub.WeeklyUsageUSD = 0
		r.sub.WeeklyWindowStart = &windowStart
	}
	return r.resetWeeklyErr
}

func (r *resetQuotaUserSubRepoStub) ResetMonthlyUsage(_ context.Context, _ int64, windowStart time.Time) error {
	r.resetMonthlyCalled = true
	if r.resetMonthlyErr == nil && r.sub != nil {
		r.sub.MonthlyUsageUSD = 0
		r.sub.MonthlyWindowStart = &windowStart
	}
	return r.resetMonthlyErr
}

func (r *resetQuotaUserSubRepoStub) ExtendExpiry(_ context.Context, _ int64, newExpiresAt time.Time) error {
	r.extendExpiryCalled = true
	r.extendedTo = newExpiresAt
	if r.extendExpiryErr == nil && r.sub != nil {
		r.sub.ExpiresAt = newExpiresAt
	}
	return r.extendExpiryErr
}

func (r *resetQuotaUserSubRepoStub) UpdateStatus(_ context.Context, _ int64, status string) error {
	r.updateStatusCalled = true
	r.updatedStatusTo = status
	if r.updateStatusErr == nil && r.sub != nil {
		r.sub.Status = status
	}
	return r.updateStatusErr
}

func newResetQuotaSvc(stub *resetQuotaUserSubRepoStub) *SubscriptionService {
	return NewSubscriptionService(groupRepoNoop{}, stub, nil, nil, nil)
}

func TestAdminResetQuota_ResetBoth(t *testing.T) {
	stub := &resetQuotaUserSubRepoStub{
		sub: &UserSubscription{ID: 1, UserID: 10, GroupID: 20},
	}
	svc := newResetQuotaSvc(stub)

	result, err := svc.AdminResetQuota(context.Background(), 1, true, true, false)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, stub.resetDailyCalled, "应调用 ResetDailyUsage")
	require.True(t, stub.resetWeeklyCalled, "应调用 ResetWeeklyUsage")
	require.False(t, stub.resetMonthlyCalled, "不应调用 ResetMonthlyUsage")
}

func TestAdminResetQuota_ResetDailyOnly(t *testing.T) {
	stub := &resetQuotaUserSubRepoStub{
		sub: &UserSubscription{ID: 2, UserID: 10, GroupID: 20},
	}
	svc := newResetQuotaSvc(stub)

	result, err := svc.AdminResetQuota(context.Background(), 2, true, false, false)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, stub.resetDailyCalled, "应调用 ResetDailyUsage")
	require.False(t, stub.resetWeeklyCalled, "不应调用 ResetWeeklyUsage")
	require.False(t, stub.resetMonthlyCalled, "不应调用 ResetMonthlyUsage")
}

func TestAdminResetQuota_ResetWeeklyOnly(t *testing.T) {
	stub := &resetQuotaUserSubRepoStub{
		sub: &UserSubscription{ID: 3, UserID: 10, GroupID: 20},
	}
	svc := newResetQuotaSvc(stub)

	result, err := svc.AdminResetQuota(context.Background(), 3, false, true, false)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, stub.resetDailyCalled, "不应调用 ResetDailyUsage")
	require.True(t, stub.resetWeeklyCalled, "应调用 ResetWeeklyUsage")
	require.False(t, stub.resetMonthlyCalled, "不应调用 ResetMonthlyUsage")
}

func TestAdminResetQuota_BothFalseReturnsError(t *testing.T) {
	stub := &resetQuotaUserSubRepoStub{
		sub: &UserSubscription{ID: 7, UserID: 10, GroupID: 20},
	}
	svc := newResetQuotaSvc(stub)

	_, err := svc.AdminResetQuota(context.Background(), 7, false, false, false)

	require.ErrorIs(t, err, ErrInvalidInput)
	require.False(t, stub.resetDailyCalled)
	require.False(t, stub.resetWeeklyCalled)
	require.False(t, stub.resetMonthlyCalled)
}

func TestAdminResetQuota_SubscriptionNotFound(t *testing.T) {
	stub := &resetQuotaUserSubRepoStub{sub: nil}
	svc := newResetQuotaSvc(stub)

	_, err := svc.AdminResetQuota(context.Background(), 999, true, true, true)

	require.ErrorIs(t, err, ErrSubscriptionNotFound)
	require.False(t, stub.resetDailyCalled)
	require.False(t, stub.resetWeeklyCalled)
	require.False(t, stub.resetMonthlyCalled)
}

func TestAdminResetQuota_ResetDailyUsageError(t *testing.T) {
	dbErr := errors.New("db error")
	stub := &resetQuotaUserSubRepoStub{
		sub:           &UserSubscription{ID: 4, UserID: 10, GroupID: 20},
		resetDailyErr: dbErr,
	}
	svc := newResetQuotaSvc(stub)

	_, err := svc.AdminResetQuota(context.Background(), 4, true, true, false)

	require.ErrorIs(t, err, dbErr)
	require.True(t, stub.resetDailyCalled)
	require.False(t, stub.resetWeeklyCalled, "daily 失败后不应继续调用 weekly")
}

func TestAdminResetQuota_ResetWeeklyUsageError(t *testing.T) {
	dbErr := errors.New("db error")
	stub := &resetQuotaUserSubRepoStub{
		sub:            &UserSubscription{ID: 5, UserID: 10, GroupID: 20},
		resetWeeklyErr: dbErr,
	}
	svc := newResetQuotaSvc(stub)

	_, err := svc.AdminResetQuota(context.Background(), 5, false, true, false)

	require.ErrorIs(t, err, dbErr)
	require.True(t, stub.resetWeeklyCalled)
}

func TestAdminResetQuota_ResetMonthlyOnly(t *testing.T) {
	stub := &resetQuotaUserSubRepoStub{
		sub: &UserSubscription{ID: 8, UserID: 10, GroupID: 20},
	}
	svc := newResetQuotaSvc(stub)

	result, err := svc.AdminResetQuota(context.Background(), 8, false, false, true)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, stub.resetDailyCalled, "不应调用 ResetDailyUsage")
	require.False(t, stub.resetWeeklyCalled, "不应调用 ResetWeeklyUsage")
	require.True(t, stub.resetMonthlyCalled, "应调用 ResetMonthlyUsage")
}

func TestAdminResetQuota_ResetMonthlyUsageError(t *testing.T) {
	dbErr := errors.New("db error")
	stub := &resetQuotaUserSubRepoStub{
		sub:             &UserSubscription{ID: 9, UserID: 10, GroupID: 20},
		resetMonthlyErr: dbErr,
	}
	svc := newResetQuotaSvc(stub)

	_, err := svc.AdminResetQuota(context.Background(), 9, false, false, true)

	require.ErrorIs(t, err, dbErr)
	require.True(t, stub.resetMonthlyCalled)
}

func TestAdminResetQuota_ReturnsRefreshedSub(t *testing.T) {
	stub := &resetQuotaUserSubRepoStub{
		sub: &UserSubscription{
			ID:            6,
			UserID:        10,
			GroupID:       20,
			DailyUsageUSD: 99.9,
		},
	}

	svc := newResetQuotaSvc(stub)
	result, err := svc.AdminResetQuota(context.Background(), 6, true, false, false)

	require.NoError(t, err)
	// ResetDailyUsage stub 会将 sub.DailyUsageUSD 归零，
	// 服务应返回第二次 GetByID 的刷新值而非初始的 99.9
	require.Equal(t, float64(0), result.DailyUsageUSD, "返回的订阅应反映已归零的用量")
	require.True(t, stub.resetDailyCalled)
}

func TestResetAllUsageWindows_AllCalled(t *testing.T) {
	stub := &resetQuotaUserSubRepoStub{
		sub: &UserSubscription{ID: 10, UserID: 10, GroupID: 20},
	}
	svc := newResetQuotaSvc(stub)
	windowStart := startOfDay(time.Now())

	err := svc.resetAllUsageWindows(context.Background(), 10, windowStart)

	require.NoError(t, err)
	require.True(t, stub.resetDailyCalled, "应调用 ResetDailyUsage")
	require.True(t, stub.resetWeeklyCalled, "应调用 ResetWeeklyUsage")
	require.True(t, stub.resetMonthlyCalled, "应调用 ResetMonthlyUsage")
}

func TestResetAllUsageWindows_StopsOnWeeklyError(t *testing.T) {
	dbErr := errors.New("weekly failed")
	stub := &resetQuotaUserSubRepoStub{
		sub:            &UserSubscription{ID: 11, UserID: 10, GroupID: 20},
		resetWeeklyErr: dbErr,
	}
	svc := newResetQuotaSvc(stub)
	windowStart := startOfDay(time.Now())

	err := svc.resetAllUsageWindows(context.Background(), 11, windowStart)

	require.ErrorIs(t, err, dbErr)
	require.True(t, stub.resetDailyCalled, "weekly 前应先调用 daily")
	require.True(t, stub.resetWeeklyCalled, "应调用 weekly")
	require.False(t, stub.resetMonthlyCalled, "weekly 失败后不应继续调用 monthly")
}

func TestExtendSubscription_PositiveDaysResetsAllWindows(t *testing.T) {
	originalExpiresAt := time.Now().Add(10 * 24 * time.Hour)
	stub := &resetQuotaUserSubRepoStub{
		sub: &UserSubscription{
			ID:              12,
			UserID:          10,
			GroupID:         20,
			Status:          SubscriptionStatusActive,
			ExpiresAt:       originalExpiresAt,
			DailyUsageUSD:   30,
			WeeklyUsageUSD:  40,
			MonthlyUsageUSD: 50,
		},
	}
	svc := newResetQuotaSvc(stub)

	result, err := svc.ExtendSubscription(context.Background(), 12, 1)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, stub.extendExpiryCalled, "应调用 ExtendExpiry")
	require.True(t, stub.resetDailyCalled, "应调用 ResetDailyUsage")
	require.True(t, stub.resetWeeklyCalled, "应调用 ResetWeeklyUsage")
	require.True(t, stub.resetMonthlyCalled, "应调用 ResetMonthlyUsage")
	require.Equal(t, float64(0), result.DailyUsageUSD)
	require.Equal(t, float64(0), result.WeeklyUsageUSD)
	require.Equal(t, float64(0), result.MonthlyUsageUSD)
	require.True(t, result.ExpiresAt.After(originalExpiresAt))
}

func TestExtendSubscription_NegativeDaysDoesNotResetWindows(t *testing.T) {
	stub := &resetQuotaUserSubRepoStub{
		sub: &UserSubscription{
			ID:              13,
			UserID:          10,
			GroupID:         20,
			Status:          SubscriptionStatusActive,
			ExpiresAt:       time.Now().Add(10 * 24 * time.Hour),
			DailyUsageUSD:   30,
			WeeklyUsageUSD:  40,
			MonthlyUsageUSD: 50,
		},
	}
	svc := newResetQuotaSvc(stub)

	_, err := svc.ExtendSubscription(context.Background(), 13, -1)

	require.NoError(t, err)
	require.False(t, stub.resetDailyCalled, "缩短订阅不应重置 daily")
	require.False(t, stub.resetWeeklyCalled, "缩短订阅不应重置 weekly")
	require.False(t, stub.resetMonthlyCalled, "缩短订阅不应重置 monthly")
}
