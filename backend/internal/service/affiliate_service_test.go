//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type affiliateSettingRepoStub struct {
	value  string
	values map[string]string
	err    error
}

func (s *affiliateSettingRepoStub) Get(context.Context, string) (*Setting, error) { return nil, s.err }
func (s *affiliateSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if s.values != nil {
		return s.values[key], nil
	}
	return s.value, nil
}
func (s *affiliateSettingRepoStub) Set(context.Context, string, string) error { return s.err }
func (s *affiliateSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return map[string]string{}, nil
}
func (s *affiliateSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return s.err
}
func (s *affiliateSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return map[string]string{}, nil
}
func (s *affiliateSettingRepoStub) Delete(context.Context, string) error { return s.err }

func TestAffiliateRebateRatePercentSemantics(t *testing.T) {
	t.Parallel()

	svc := &AffiliateService{settingRepo: &affiliateSettingRepoStub{value: "1"}}
	rate := svc.loadAffiliateRebateRatePercent(context.Background())
	require.Equal(t, 1.0, rate)

	svc.settingRepo = &affiliateSettingRepoStub{value: "0.2"}
	rate = svc.loadAffiliateRebateRatePercent(context.Background())
	require.Equal(t, 0.2, rate)

	svc.settingRepo = &affiliateSettingRepoStub{err: context.Canceled}
	rate = svc.loadAffiliateRebateRatePercent(context.Background())
	require.Equal(t, AffiliateRebateRateDefault, rate)
}

type affiliateRepoStub struct {
	summary           *AffiliateSummary
	codeSummary       *AffiliateSummary
	bindCalled        bool
	bindUserID        int64
	bindInviterID     int64
	bindSignupReward  float64
	bindResult        bool
	bindErr           error
	transferCalled    bool
	transferUserID    int64
	transferMinAmount float64
	transferAmount    float64
	transferBalance   float64
	transferErr       error
	invitees          []AffiliateInvitee
}

func (s *affiliateRepoStub) EnsureUserAffiliate(context.Context, int64) (*AffiliateSummary, error) {
	return s.summary, nil
}

func (s *affiliateRepoStub) GetAffiliateByCode(context.Context, string) (*AffiliateSummary, error) {
	return s.codeSummary, nil
}

func (s *affiliateRepoStub) BindInviter(_ context.Context, userID, inviterID int64, signupReward float64) (bool, error) {
	s.bindCalled = true
	s.bindUserID = userID
	s.bindInviterID = inviterID
	s.bindSignupReward = signupReward
	return s.bindResult, s.bindErr
}

func (s *affiliateRepoStub) AccrueQuota(context.Context, int64, int64, float64) (bool, error) {
	return true, nil
}

func (s *affiliateRepoStub) TransferQuotaToBalance(_ context.Context, userID int64, minAmount float64) (float64, float64, error) {
	s.transferCalled = true
	s.transferUserID = userID
	s.transferMinAmount = minAmount
	return s.transferAmount, s.transferBalance, s.transferErr
}

func (s *affiliateRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	return s.invitees, nil
}

func TestBindInviterByCode_UsesSignupRewardSetting(t *testing.T) {
	t.Parallel()

	repo := &affiliateRepoStub{
		summary: &AffiliateSummary{
			UserID:  2,
			AffCode: "SELFCODE0001",
		},
		codeSummary: &AffiliateSummary{
			UserID:  1,
			AffCode: "ABCDEFGHJKLM",
		},
		bindResult: true,
	}
	svc := &AffiliateService{
		repo:        repo,
		settingRepo: &affiliateSettingRepoStub{value: "3"},
	}

	err := svc.BindInviterByCode(context.Background(), 2, "ABCDEFGHJKLM")
	require.NoError(t, err)
	require.True(t, repo.bindCalled)
	require.Equal(t, int64(2), repo.bindUserID)
	require.Equal(t, int64(1), repo.bindInviterID)
	require.Equal(t, 3.0, repo.bindSignupReward)
}

func TestTransferAffiliateQuota_RequiresThreshold(t *testing.T) {
	t.Parallel()

	repo := &affiliateRepoStub{
		summary: &AffiliateSummary{
			UserID:   1,
			AffCode:  "ABCDEFGHJKLM",
			AffQuota: 4.5,
		},
	}
	svc := &AffiliateService{
		repo:        repo,
		settingRepo: &affiliateSettingRepoStub{value: "5"},
	}

	_, _, err := svc.TransferAffiliateQuota(context.Background(), 1)
	require.ErrorIs(t, err, ErrAffiliateQuotaThreshold)
	require.False(t, repo.transferCalled)
}

func TestGetAffiliateDetail_ExposesProgramSettings(t *testing.T) {
	t.Parallel()

	repo := &affiliateRepoStub{
		summary: &AffiliateSummary{
			UserID:          1,
			AffCode:         "ABCDEFGHJKLM",
			AffQuota:        3,
			AffHistoryQuota: 7,
		},
	}
	svc := &AffiliateService{
		repo: repo,
		settingRepo: &affiliateSettingRepoStub{values: map[string]string{
			SettingKeyAffiliateRebateRate:        "10",
			SettingKeyAffiliateSignupReward:      "3",
			SettingKeyAffiliateTransferThreshold: "5",
		}},
	}

	detail, err := svc.GetAffiliateDetail(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 10.0, detail.RebateRate)
	require.Equal(t, AffiliateSignupRewardDefault, detail.SignupReward)
	require.Equal(t, AffiliateTransferThresholdDefault, detail.TransferMin)
	require.Equal(t, 2.0, detail.TransferGap)
	require.False(t, detail.CanTransfer)
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	require.Equal(t, "a***@g***.com", maskEmail("alice@gmail.com"))
	require.Equal(t, "x***@d***", maskEmail("x@domain"))
	require.Equal(t, "", maskEmail(""))
}

func TestIsValidAffiliateCodeFormat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid canonical", "ABCDEFGHJKLM", true},
		{"valid all digits 2-9", "234567892345", true},
		{"valid mixed", "A2B3C4D5E6F7", true},
		{"too short", "ABCDEFGHJKL", false},
		{"too long", "ABCDEFGHJKLMN", false},
		{"contains excluded letter I", "IBCDEFGHJKLM", false},
		{"contains excluded letter O", "OBCDEFGHJKLM", false},
		{"contains excluded digit 0", "0BCDEFGHJKLM", false},
		{"contains excluded digit 1", "1BCDEFGHJKLM", false},
		{"lowercase rejected (caller must ToUpper first)", "abcdefghjklm", false},
		{"empty", "", false},
		{"12-byte utf8 non-ascii", "ÄÄÄÄÄÄ", false}, // 6×2 bytes = 12 bytes, bytes out of charset
		{"ascii punctuation", "ABCDEFGHJK.M", false},
		{"whitespace", "ABCDEFGHJK M", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isValidAffiliateCodeFormat(tc.in))
		})
	}
}
