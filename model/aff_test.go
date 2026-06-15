package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestCalculateAffRechargeRebateQuota(t *testing.T) {
	require.Equal(t, 50, CalculateAffRechargeRebateQuota(1000))
	require.Equal(t, 0, CalculateAffRechargeRebateQuota(19))
	require.Equal(t, 0, CalculateAffRechargeRebateQuota(0))
}

func TestRedeemGrantsAffRechargeRebate(t *testing.T) {
	truncateTables(t)

	inviter := &User{
		Id:       201,
		Username: "aff_inviter",
		Status:   common.UserStatusEnabled,
		AffCode:  "A201",
	}
	invitee := &User{
		Id:        202,
		Username:  "aff_invitee",
		Status:    common.UserStatusEnabled,
		AffCode:   "A202",
		InviterId: inviter.Id,
	}
	require.NoError(t, DB.Create(inviter).Error)
	require.NoError(t, DB.Create(invitee).Error)

	redemption := &Redemption{
		Key:    "12345678901234567890123456789012",
		Status: common.RedemptionCodeStatusEnabled,
		Name:   "aff-redemption",
		Quota:  1000,
	}
	require.NoError(t, DB.Create(redemption).Error)

	quota, err := Redeem(redemption.Key, invitee.Id)
	require.NoError(t, err)
	require.Equal(t, 1000, quota)

	var updatedInviter User
	require.NoError(t, DB.First(&updatedInviter, inviter.Id).Error)
	require.Equal(t, 50, updatedInviter.AffQuota)
	require.Equal(t, 50, updatedInviter.AffHistoryQuota)

	var updatedInvitee User
	require.NoError(t, DB.First(&updatedInvitee, invitee.Id).Error)
	require.Equal(t, 1000, updatedInvitee.Quota)
}

func TestInsertWithInviterIncrementsAffCountWhenInviterQuotaIsZero(t *testing.T) {
	truncateTables(t)
	configureInviteRegistrationTest(t, 0)

	inviter := &User{
		Id:       301,
		Username: "invite_count_zero_inviter",
		Status:   common.UserStatusEnabled,
		AffCode:  "IC01",
	}
	require.NoError(t, DB.Create(inviter).Error)

	invitee := &User{
		Username:    "invite_count_zero_invitee",
		Password:    "password123",
		DisplayName: "invite_count_zero_invitee",
		InviterId:   inviter.Id,
		Role:        common.RoleCommonUser,
	}
	require.NoError(t, invitee.Insert(inviter.Id))

	var updatedInviter User
	require.NoError(t, DB.First(&updatedInviter, inviter.Id).Error)
	require.Equal(t, 1, updatedInviter.AffCount)
	require.Equal(t, 0, updatedInviter.AffQuota)
	require.Equal(t, 0, updatedInviter.AffHistoryQuota)
}

func TestInsertWithInviterIncrementsAffCountAndQuotaWhenInviterQuotaIsPositive(t *testing.T) {
	truncateTables(t)
	configureInviteRegistrationTest(t, 200)

	inviter := &User{
		Id:       302,
		Username: "invite_count_quota_inviter",
		Status:   common.UserStatusEnabled,
		AffCode:  "IC02",
	}
	require.NoError(t, DB.Create(inviter).Error)

	invitee := &User{
		Username:    "invite_count_quota_invitee",
		Password:    "password123",
		DisplayName: "invite_count_quota_invitee",
		InviterId:   inviter.Id,
		Role:        common.RoleCommonUser,
	}
	require.NoError(t, invitee.Insert(inviter.Id))

	var updatedInviter User
	require.NoError(t, DB.First(&updatedInviter, inviter.Id).Error)
	require.Equal(t, 1, updatedInviter.AffCount)
	require.Equal(t, 200, updatedInviter.AffQuota)
	require.Equal(t, 200, updatedInviter.AffHistoryQuota)
}

func configureInviteRegistrationTest(t *testing.T, quotaForInviter int) {
	t.Helper()

	oldQuotaForInviter := common.QuotaForInviter
	oldQuotaForInvitee := common.QuotaForInvitee
	oldQuotaForNewUser := common.QuotaForNewUser
	paymentSetting := operation_setting.GetPaymentSetting()
	oldPaymentSetting := *paymentSetting

	common.QuotaForInviter = quotaForInviter
	common.QuotaForInvitee = 0
	common.QuotaForNewUser = 0
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	t.Cleanup(func() {
		common.QuotaForInviter = oldQuotaForInviter
		common.QuotaForInvitee = oldQuotaForInvitee
		common.QuotaForNewUser = oldQuotaForNewUser
		*paymentSetting = oldPaymentSetting
	})
}
